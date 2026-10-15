package capture

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseConnectionStatus(t *testing.T) {
	for payload, want := range map[string]ConnectionStatus{
		"true":  {Connected: true},
		"false": {},
		`{"connected":true,"kind":"capture-mod","version":"1.0.0"}`: {Connected: true, Kind: KindMod, Version: "1.0.0"},
		`{"connected":false}`: {},
	} {
		got, err := ParseConnectionStatus(payload)
		if err != nil || got != want {
			t.Errorf("%s: got %+v, %v; want %+v", payload, got, err, want)
		}
	}
	if _, err := ParseConnectionStatus("yes"); err == nil {
		t.Error("a malformed payload should be an error")
	}
}

func TestConnectionStatus_RoundTrips(t *testing.T) {
	for _, c := range []ConnectionStatus{{}, {Connected: true}, {Connected: true, Kind: KindApp, Version: "6.0.0"}} {
		got, err := ParseConnectionStatus(c.Payload())
		if err != nil || got != c {
			t.Errorf("%+v: round-tripped to %+v, %v", c, got, err)
		}
	}
}

func TestIdentityFromQuery(t *testing.T) {
	kind, version := IdentityFromQuery(url.Values{"kind": {" capture-mod "}, "version": {"1.2.3"}})
	if kind != KindMod || version != "1.2.3" {
		t.Errorf("got %q %q", kind, version)
	}
	kind, version = IdentityFromQuery(url.Values{"EIO": {"3"}})
	if kind != "" || version != "" {
		t.Errorf("a capture client that reports nothing got %q %q", kind, version)
	}
	if kind, _ := IdentityFromQuery(url.Values{"kind": {strings.Repeat("x", maxIdentityLength+1)}}); kind != "" {
		t.Errorf("an overlong kind was kept: %q", kind)
	}
}
