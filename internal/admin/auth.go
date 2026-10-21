package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	iterations  = 600000
	cookieName  = "locas_session"
	sessionLife = 8 * time.Hour
)

type credentials struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}

type session struct {
	Expires time.Time
}

type Manager struct {
	mu       sync.Mutex
	path     string
	creds    *credentials
	sessions map[string]session
	failures map[string][]time.Time
}

func New(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	m := &Manager{
		path:     filepath.Join(dir, "admin.json"),
		sessions: make(map[string]session),
		failures: make(map[string][]time.Time),
	}
	data, err := os.ReadFile(m.path)
	if err == nil {
		var c credentials
		if json.Unmarshal(data, &c) != nil {
			return nil, errors.New("invalid administrator state")
		}
		salt, e1 := base64.RawStdEncoding.DecodeString(c.Salt)
		hash, e2 := base64.RawStdEncoding.DecodeString(c.Hash)
		if e1 != nil || e2 != nil || len(salt) != 32 || len(hash) != 32 {
			return nil, errors.New("invalid administrator credential encoding")
		}
		m.creds = &c
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return m, nil
}

func derive(password string, salt []byte) []byte {
	// PBKDF2-HMAC-SHA256, RFC 8018, 600,000 iterations, 32-byte derived key.
	var counter = []byte{0, 0, 0, 1}
	mac := hmac.New(sha256.New, []byte(password))
	_, _ = mac.Write(salt)
	_, _ = mac.Write(counter)
	u := mac.Sum(nil)
	out := append([]byte(nil), u...)
	for i := 1; i < iterations; i++ {
		mac.Reset()
		_, _ = mac.Write(u)
		u = mac.Sum(u[:0])
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}

func (m *Manager) Configured() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.creds != nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (m *Manager) Setup(password string) error {
	if len(password) < 14 || len(password) > 256 {
		return errors.New("password must contain 14 to 256 characters")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.creds != nil {
		return errors.New("administrator already configured")
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	c := credentials{
		Salt: base64.RawStdEncoding.EncodeToString(salt),
		Hash: base64.RawStdEncoding.EncodeToString(derive(password, salt)),
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(m.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// Never allow an incomplete credentials file to be treated as configured.
		_ = os.Remove(m.path)
		return err
	}
	m.creds = &c
	return nil
}

func (m *Manager) Login(password, remote string) (string, bool) {
	remoteHost, _, err := net.SplitHostPort(remote)
	if err == nil {
		remote = remoteHost
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.creds == nil {
		return "", false
	}
	// Limit guessing from the local interface (including multiple browser tabs).
	recent := m.failures[remote][:0]
	for _, t := range m.failures[remote] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	m.failures[remote] = recent
	if len(recent) >= 5 {
		return "", false
	}
	salt, _ := base64.RawStdEncoding.DecodeString(m.creds.Salt)
	expected, _ := base64.RawStdEncoding.DecodeString(m.creds.Hash)
	actual := derive(password, salt)
	if subtle.ConstantTimeCompare(expected, actual) != 1 {
		m.failures[remote] = append(m.failures[remote], now)
		return "", false
	}
	delete(m.failures, remote)
	key, err := randomString(32)
	if err != nil {
		return "", false
	}
	m.sessions[key] = session{Expires: now.Add(sessionLife)}
	return key, true
}

func (m *Manager) Authenticated(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(s.Expires) {
		delete(m.sessions, c.Value)
		return false
	}
	return true
}

func (m *Manager) Logout(r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err == nil {
		m.mu.Lock()
		delete(m.sessions, c.Value)
		m.mu.Unlock()
	}
}

func SetCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: secure, MaxAge: int(sessionLife.Seconds()),
	})
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

func SafeMutation(r *http.Request) bool {
	if r.Header.Get("X-Locas-Request") != "1" {
		return false
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return false
	}
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "http://"+r.Host
}

func DecodePassword(w http.ResponseWriter, r *http.Request) (string, error) {
	var body struct {
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", errors.New("unexpected extra JSON")
	}
	return body.Password, nil
}
