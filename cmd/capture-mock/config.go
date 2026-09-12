package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/capture"
)

// Intentionally fixed in source: this development tool should not make it easy
// to impersonate a capture client against a live deployment.
const galactusHost = "http://localhost:8123"

// Links validate the local endpoint; they never select the connection target.
func parseConnection(input string) (string, string, error) {
	code := strings.TrimSpace(input)
	if strings.Contains(code, "://") {
		link, err := url.Parse(code)
		if err != nil || link.Scheme != "aucapture" || link.Hostname() == "" || link.User != nil || link.Fragment != "" {
			return "", "", fmt.Errorf("expected an aucapture://host/CODE link")
		}
		query, err := url.ParseQuery(link.RawQuery)
		if err != nil {
			return "", "", fmt.Errorf("invalid capture link query: %w", err)
		}
		for key := range query {
			if key != "insecure" {
				return "", "", fmt.Errorf("unsupported capture link parameter %q", key)
			}
		}
		if link.Host != "localhost:8123" || !query.Has("insecure") || query.Get("insecure") != "" {
			return "", "", fmt.Errorf("capture mock only accepts local links: aucapture://localhost:8123/CODE?insecure")
		}
		code = strings.TrimPrefix(link.Path, "/")
	}
	code = strings.ToUpper(code)
	if len(code) != capture.ConnectCodeLength || strings.ContainsAny(code, "/?# \\ \t\r\n") {
		return "", "", fmt.Errorf("connect code must be %d characters with no whitespace or URL separators", capture.ConnectCodeLength)
	}
	return galactusHost, code, nil
}
