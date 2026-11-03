package updateapproval

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func issue(t *testing.T, s *Store, session string) Selection {
	t.Helper()
	g, err := s.Begin(session)
	if err != nil {
		t.Fatal(err)
	}
	approval, _, err := s.Issue(session, g, "v1.2.3", "stable", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	return approval
}
func TestSessionTamperExpiryAndReplay(t *testing.T) {
	s := NewStore()
	now := time.Now()
	s.now = func() time.Time { return now }
	a := issue(t, s, "one")
	if _, err := s.Consume("two", a); err == nil {
		t.Fatal("cross-session approval accepted")
	}
	wrong := a
	wrong.SHA256 = strings.Repeat("b", 64)
	if _, err := s.Consume("one", wrong); err == nil {
		t.Fatal("digest substitution accepted")
	}
	wrong = a
	wrong.Channel = "beta"
	if _, err := s.Consume("one", wrong); err == nil {
		t.Fatal("channel substitution accepted")
	}
	if _, err := s.Consume("one", a); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Consume("one", a); err == nil {
		t.Fatal("replay accepted")
	}
	a = issue(t, s, "one")
	now = now.Add(Lifetime)
	if _, err := s.Consume("one", a); err == nil {
		t.Fatal("expiry boundary accepted")
	}
}
func TestConcurrentConsumeExactlyOnce(t *testing.T) {
	s := NewStore()
	a := issue(t, s, "one")
	var count atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Consume("one", a); err == nil {
				count.Add(1)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("%d attempts authorized", count.Load())
	}
}
func TestPendingDiscoveryInvalidation(t *testing.T) {
	for _, kind := range []string{"logout", "channel", "recheck", "restart"} {
		t.Run(kind, func(t *testing.T) {
			s := NewStore()
			old, err := s.Begin("one")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "logout":
				s.Invalidate("one")
			case "channel":
				s.InvalidateAll()
			case "recheck":
				s.Begin("one")
			case "restart":
				s = NewStore()
			}
			if _, _, err := s.Issue("one", old, "v1.2.3", "stable", strings.Repeat("a", 64)); err == nil {
				t.Fatal("stale discovery issued approval")
			}
			// Eviction must not reset the generation and allow an old network completion.
			s.Invalidate("one")
			if kind != "restart" {
				s.Begin("one")
				if _, _, err := s.Issue("one", old, "v1.2.3", "stable", strings.Repeat("a", 64)); err == nil {
					t.Fatal("generation resurrected")
				}
			}
		})
	}
}
func TestInvalidateAllSessions(t *testing.T) {
	s := NewStore()
	a := issue(t, s, "one")
	b := issue(t, s, "two")
	s.InvalidateAll()
	if _, err := s.Consume("one", a); err == nil {
		t.Fatal("first session retained")
	}
	if _, err := s.Consume("two", b); err == nil {
		t.Fatal("other session retained")
	}
}
func TestBoundedCapacityAndExpiredEviction(t *testing.T) {
	s := NewStore()
	now := time.Now()
	s.now = func() time.Time { return now }
	for i := 0; i < 256; i++ {
		if _, err := s.Begin(string(rune(i + 1))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Begin("overflow"); err == nil {
		t.Fatal("unbounded selection state")
	}
	now = now.Add(Lifetime)
	if _, err := s.Begin("fresh"); err != nil {
		t.Fatal(err)
	}
}
func TestJobStrictValidation(t *testing.T) {
	s := NewStore()
	a := issue(t, s, "one")
	now := time.Now()
	for _, expiry := range []time.Time{now, now.Add(-time.Second), now.Add(Lifetime + time.Second)} {
		if (Job{Selection: a, ExpiresAt: expiry}).Validate(now) == nil {
			t.Fatal("invalid lifetime")
		}
	}
	for _, body := range []string{`{} {}`, `{"command":"shell"}`, `null garbage`} {
		var job Job
		if Decode(strings.NewReader(body), &job) == nil {
			t.Fatal("malformed job accepted")
		}
	}
}
