package hardware

import "testing"

func TestParseMeminfo(t *testing.T) {
	total, free, err := ParseMeminfo("MemTotal: 32768 kB\nMemAvailable: 16384 kB\n")
	if err != nil || total != 32 || free != 16 {
		t.Fatalf("%d %d %v", total, free, err)
	}
}
func TestRejectInvalid(t *testing.T) {
	for _, v := range []string{"MemTotal: 100 kB\n", "MemTotal: 100 kB\nMemAvailable: 200 kB\n", "MemTotal: invalid kB\nMemAvailable: 4 kB\n"} {
		if _, _, err := ParseMeminfo(v); err == nil {
			t.Errorf("accepted invalid %q", v)
		}
	}
}
