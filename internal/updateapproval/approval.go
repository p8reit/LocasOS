// Package updateapproval defines the artifact identity carried across the
// authenticated core, restricted agent queue and independent update worker.
package updateapproval

import (
	"encoding/json"
	"errors"
	"github.com/p8reit/LocasOS/internal/auditop"
	"github.com/p8reit/LocasOS/internal/updatetrust"
	"io"
	"regexp"
	"time"
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

const Lifetime = 10 * time.Minute

type Selection struct {
	Version string `json:"version"`
	Channel string `json:"channel"`
	SHA256  string `json:"sha256"`
	Token   string `json:"token"`
}

func (s Selection) Validate() error {
	if _, ok := updatetrust.VersionParts(s.Version); !ok || (s.Channel != "stable" && s.Channel != "beta") || !hexPattern.MatchString(s.SHA256) || !hexPattern.MatchString(s.Token) {
		return errors.New("invalid artifact approval")
	}
	return nil
}

// Token identifies one authenticated enqueue attempt. It is not independently
// trusted root authorization; the worker still verifies its own publisher key.
type Job struct {
	Selection
	ExpiresAt time.Time           `json:"expires_at"`
	Audit     auditop.Attribution `json:"audit"`
}

func (j Job) Validate(now time.Time) error {
	if err := j.Selection.Validate(); err != nil {
		return err
	}
	if !now.Before(j.ExpiresAt) || j.ExpiresAt.After(now.Add(Lifetime)) {
		return errors.New("expired or invalid approval lifetime")
	}
	return j.Audit.Validate(false)
}

// Decode rejects unknown fields and trailing JSON at every privilege boundary.
func Decode(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}
