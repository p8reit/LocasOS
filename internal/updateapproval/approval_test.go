package updateapproval

import (
	"strings"
	"testing"
)

func TestSelectionUsesSharedNumericVersionBounds(t *testing.T) {
	s := Selection{Channel: "stable", SHA256: strings.Repeat("a", 64), Token: strings.Repeat("b", 64)}
	for _, v := range []string{"v1.2.3", "v9223372036854775807.0.0", "v00000000000000000000000000001.2.3"} {
		s.Version = v
		if err := s.Validate(); err != nil {
			t.Fatalf("valid %s: %v", v, err)
		}
	}
	for _, v := range []string{"v9223372036854775808.0.0", "v" + strings.Repeat("0", 64) + "1.0.0", "v1.bad.0"} {
		s.Version = v
		if s.Validate() == nil {
			t.Fatalf("invalid %s accepted", v)
		}
	}
}
