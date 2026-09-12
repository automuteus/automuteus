package main

import (
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/capture"
)

func TestParseConnection(t *testing.T) {
	// The former override must have no effect, even for a bare code.
	t.Setenv("GALACTUS_HOST", "https://live.example:443")
	link, _, _ := capture.FormCaptureURL("http://localhost:8123", "", "ABCDEFGH")
	for _, input := range []string{" abcdefgh ", "aucapture://localhost:8123/abcdefgh?insecure", link} {
		host, code, err := parseConnection(input)
		if err != nil || host != "http://localhost:8123" || code != "ABCDEFGH" {
			t.Errorf("input %q: got (%q, %q, %v), want local host and ABCDEFGH", input, host, code, err)
		}
	}
}

func TestParseConnectionRejectsNonLocalEndpoints(t *testing.T) {
	for _, input := range []string{
		"aucapture://live.example:8123/ABCDEFGH?insecure",
		"aucapture://live.example:443/ABCDEFGH",
		"aucapture://localhost:9999/ABCDEFGH?insecure",
		"aucapture://localhost/ABCDEFGH?insecure",
		"aucapture://localhost:8123/ABCDEFGH",
		"aucapture://localhost:8123/ABCDEFGH?insecure=false",
		"aucapture://localhost.evil.example:8123/ABCDEFGH?insecure",
		"aucapture://localhost:8123@live.example:8123/ABCDEFGH?insecure",
		"aucapture://127.0.0.1:8123/ABCDEFGH?insecure",
		"aucapture://[::1]:8123/ABCDEFGH?insecure",
	} {
		if _, _, err := parseConnection(input); err == nil {
			t.Errorf("accepted endpoint override %q", input)
		}
	}
}

func TestParseConnectionRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "SHORT", "TOOLONG99", "ABCD EFG", "ABCD/EFG", "aucapture://localhost:8123/?insecure", "aucapture://localhost:8123/ABCDEFGH/?insecure", "aucapture:///ABCDEFGH", "http://localhost:8123/ABCDEFGH", "aucapture://user:pass@localhost:8123/ABCDEFGH?insecure", "aucapture://localhost:8123/ABCDEFGH?insecure#fragment", "aucapture://localhost:8123/ABCDEFGH?insecure;%", "aucapture://localhost:8123/ABCDEFGH?insecure&other=true", "aucapture://localhost:bad/ABCDEFGH"} {
		if _, _, err := parseConnection(input); err == nil {
			t.Errorf("accepted invalid input %q", input)
		}
	}
}
