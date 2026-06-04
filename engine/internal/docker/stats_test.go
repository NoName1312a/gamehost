package docker

import (
	"math"
	"testing"
)

func TestParseMemUsage(t *testing.T) {
	cases := []struct {
		in              string
		wantUsed, wantL float64
	}{
		{"512MiB / 4GiB", 512, 4096},
		{"512.0MiB / 3.844GiB", 512, 3936.256},
		{"900kB / 1.5GiB", 900.0 / 1000, 1536},
		{"1.5GiB / 2GiB", 1536, 2048},
	}
	for _, c := range cases {
		used, limit := parseMemUsage(c.in)
		if math.Abs(used-c.wantUsed) > 0.01 || math.Abs(limit-c.wantL) > 0.01 {
			t.Errorf("parseMemUsage(%q) = (%.3f, %.3f), want (%.3f, %.3f)", c.in, used, limit, c.wantUsed, c.wantL)
		}
	}
	if p := parsePercent("12.34%"); math.Abs(p-12.34) > 0.001 {
		t.Errorf("parsePercent = %v", p)
	}
}
