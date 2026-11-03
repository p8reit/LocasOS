package updateprefs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileName = "update-channel"

// ValidChannel deliberately restricts updates to published stable releases
// or an administrator-selected Early Access release channel.
func ValidChannel(channel string) bool {
	return channel == "stable" || channel == "beta"
}

func Load(stateDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, fileName))
	if errors.Is(err, os.ErrNotExist) {
		return "stable", nil
	}
	if err != nil {
		return "stable", fmt.Errorf("read update channel: %w", err)
	}
	channel := strings.TrimSpace(string(data))
	if !ValidChannel(channel) {
		return "stable", errors.New("invalid saved update channel")
	}
	return channel, nil
}

// Save creates a mode-0600 preference file and atomically replaces the previous
// value. The state directory belongs to the unprivileged LOCAS Core account.
func Save(stateDir, channel string) error {
	if !ValidChannel(channel) {
		return errors.New("unsupported update channel")
	}
	file, err := os.CreateTemp(stateDir, ".update-channel-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.WriteString(channel + "\n"); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(stateDir, fileName))
}
