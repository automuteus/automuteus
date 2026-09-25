package game

import "testing"

func TestRegionFromString_RoundTrips(t *testing.T) {
	for _, r := range []Region{NA, AS, EU} {
		got, ok := RegionFromString(r.ToString())
		if !ok || got != r {
			t.Errorf("RegionFromString(%q) = %v, %v; want %v, true", r.ToString(), got, ok, r)
		}
	}
	for _, s := range []string{"", "Unknown", Region(7).ToString()} {
		if _, ok := RegionFromString(s); ok {
			t.Errorf("RegionFromString(%q) reported ok", s)
		}
	}
}
