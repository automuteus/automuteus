package main

import (
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/capture"
)

func TestParseConnection(t *testing.T) {
	for _, tc := range []struct{ name, input, host, wantHost, wantCode string }{
		{"bare code", " abcdefgh ", "", defaultHost, "ABCDEFGH"},
		{"configured host", "ABCDEFGH", "https://broker.example:443/", "https://broker.example:443", "ABCDEFGH"},
		{"local link", "aucapture://localhost:8123/abcdefgh?insecure", "", defaultHost, "ABCDEFGH"},
		{"secure link overrides host", "aucapture://broker.example:443/ABCDEFGH", defaultHost, "https://broker.example:443", "ABCDEFGH"},
		{"IPv6", "aucapture://[::1]:8123/ABCDEFGH?insecure", "", "http://[::1]:8123", "ABCDEFGH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, code, err := parseConnection(tc.input, tc.host)
			if err != nil || host != tc.wantHost || code != tc.wantCode {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, nil)", host, code, err, tc.wantHost, tc.wantCode)
			}
		})
	}
	for _, host := range []string{"http://localhost:8123", "https://broker.example", "https://broker.example:8443"} {
		link, _, _ := capture.FormCaptureURL(host, "", "ABCDEFGH")
		if _, code, err := parseConnection(link, ""); err != nil || code != "ABCDEFGH" {
			t.Errorf("cannot parse generated link %q: code=%q, err=%v", link, code, err)
		}
	}
}

func TestParseConnectionRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "SHORT", "TOOLONG99", "ABCD EFG", "ABCD/EFG", "aucapture://localhost/", "aucapture://localhost/ABCDEFGH/", "aucapture:///ABCDEFGH", "http://localhost/ABCDEFGH", "aucapture://user:pass@localhost/ABCDEFGH", "aucapture://localhost/ABCDEFGH#fragment", "aucapture://localhost/ABCDEFGH?insecure;%", "aucapture://localhost/ABCDEFGH?other=true", "aucapture://localhost:bad/ABCDEFGH"} {
		if _, _, err := parseConnection(input, ""); err == nil {
			t.Errorf("accepted invalid input %q", input)
		}
	}
	for _, host := range []string{"localhost:8123", "ftp://localhost", "http://", "http://user:pass@localhost", "http://localhost/path", "http://localhost?query=1", "http://localhost#fragment"} {
		if _, _, err := parseConnection("ABCDEFGH", host); err == nil {
			t.Errorf("accepted invalid host %q", host)
		}
	}
}
