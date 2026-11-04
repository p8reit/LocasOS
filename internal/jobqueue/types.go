// Package jobqueue stores bounded, nonsecret compute-job metadata. It does not
// execute work or guarantee GPU isolation. Cancellation, expiry, and recovery
// release metadata reservations only; a future dispatcher must establish device
// quiescence/fencing before relying on that release for physical GPU capacity.
package jobqueue

import (
	"errors"
	"regexp"
	"time"
)

type Kind string

const (
	KindChat      Kind = "chat"
	KindModelPull Kind = "model.pull"
)

type Status string

const (
	Queued           Status = "queued"
	Running          Status = "running"
	Succeeded        Status = "succeeded"
	Failed           Status = "failed"
	Cancelled        Status = "cancelled"
	TimedOut         Status = "timed_out"
	Interrupted      Status = "interrupted"
	MaxMemoryMiB     int64  = 1 << 30
	MaxRetainedJobs         = 10000
	MaxQueuedJobs           = 1024
	MaxDatabaseBytes int64  = 16 << 20
	MaxWALBytes      int64  = 17 << 20
)

var (
	ErrInvalid    = errors.New("invalid queue metadata")
	ErrFull       = errors.New("queue capacity reached")
	ErrNotFound   = errors.New("job not found")
	ErrTransition = errors.New("invalid job transition")
	ErrOwned      = errors.New("queue directory already owned")
	ErrClosed     = errors.New("queue closed")
	ErrPoisoned   = errors.New("queue requires close and reopen after storage failure")
	idPattern     = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Request deliberately has no body, prompt, model name, credentials, or freeform
// error. IDs are opaque correlation references, never secret-bearing strings.
type Request struct {
	Kind          Kind
	CorrelationID string
	SessionID     string
	MemoryMiB     int64
	QueueTimeout  time.Duration
	RunTimeout    time.Duration
}
type Job struct {
	ID            string
	Sequence      int64
	Request       Request
	Status        Status
	QueuedAt      time.Time
	StartedAt     time.Time
	FinishedAt    time.Time
	QueueDeadline time.Time
	RunDeadline   time.Time
}
type Snapshot struct {
	TotalMiB, FreeMiB int64
	ObservedAt        time.Time
}
type Policy struct {
	BudgetMiB, HeadroomMiB int64
	MaxSnapshotAge         time.Duration
}

func validTime(t time.Time) bool {
	return !t.IsZero() && t.UnixNano() != 0 && t.Year() >= 1970 && t.Year() <= 2200
}
func (r Request) validate() error {
	if (r.Kind != KindChat && r.Kind != KindModelPull) || !idPattern.MatchString(r.CorrelationID) || !idPattern.MatchString(r.SessionID) || r.MemoryMiB <= 0 || r.MemoryMiB > MaxMemoryMiB || r.QueueTimeout <= 0 || r.QueueTimeout > 24*time.Hour || r.RunTimeout <= 0 || r.RunTimeout > time.Hour {
		return ErrInvalid
	}
	return nil
}
func (p Policy) validate() error {
	if p.BudgetMiB <= 0 || p.BudgetMiB > MaxMemoryMiB || p.HeadroomMiB < 0 || p.HeadroomMiB > MaxMemoryMiB || p.MaxSnapshotAge <= 0 || p.MaxSnapshotAge > time.Hour {
		return ErrInvalid
	}
	return nil
}
func (j Job) validate() error {
	if !idPattern.MatchString(j.ID) || j.Sequence <= 0 || j.Request.validate() != nil || !validTime(j.QueuedAt) || !validTime(j.QueueDeadline) || !j.QueueDeadline.Equal(j.QueuedAt.Add(j.Request.QueueTimeout)) {
		return ErrInvalid
	}
	switch j.Status {
	case Queued:
		if !j.StartedAt.IsZero() || !j.FinishedAt.IsZero() || !j.RunDeadline.IsZero() {
			return ErrInvalid
		}
	case Running, Succeeded, Failed, Interrupted:
		if !validTime(j.StartedAt) || j.StartedAt.Before(j.QueuedAt) || !j.StartedAt.Before(j.QueueDeadline) || (!validTime(j.RunDeadline) || !j.RunDeadline.Equal(j.StartedAt.Add(j.Request.RunTimeout))) {
			return ErrInvalid
		}
		if j.Status == Running {
			if !j.FinishedAt.IsZero() {
				return ErrInvalid
			}
		} else if !validTime(j.FinishedAt) || j.FinishedAt.Before(j.StartedAt) {
			return ErrInvalid
		}
	case Cancelled, TimedOut:
		if !validTime(j.FinishedAt) || j.FinishedAt.Before(j.QueuedAt) {
			return ErrInvalid
		}
		if j.StartedAt.IsZero() {
			if !j.RunDeadline.IsZero() {
				return ErrInvalid
			}
			if j.Status == TimedOut && j.FinishedAt.Before(j.QueueDeadline) {
				return ErrInvalid
			}
		} else {
			if !validTime(j.StartedAt) || j.StartedAt.Before(j.QueuedAt) || !j.StartedAt.Before(j.QueueDeadline) || j.FinishedAt.Before(j.StartedAt) || (!validTime(j.RunDeadline) || !j.RunDeadline.Equal(j.StartedAt.Add(j.Request.RunTimeout))) {
				return ErrInvalid
			}
			if j.Status == TimedOut && j.FinishedAt.Before(j.RunDeadline) {
				return ErrInvalid
			}
		}
	default:
		return ErrInvalid
	}
	return nil
}
