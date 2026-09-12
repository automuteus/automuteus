package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/capture"
)

const defaultHost = "http://localhost:8123"

// A capture link supplies its own endpoint. GALACTUS_HOST is used for bare codes.
func parseConnection(input, host string) (string, string, error) {
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
		scheme := "https"
		if query.Has("insecure") {
			scheme = "http"
		}
		host = scheme + "://" + link.Host
		code = strings.TrimPrefix(link.Path, "/")
	}
	code = strings.ToUpper(code)
	if len(code) != capture.ConnectCodeLength || strings.ContainsAny(code, "/?# \\ \t\r\n") {
		return "", "", fmt.Errorf("connect code must be %d characters with no whitespace or URL separators", capture.ConnectCodeLength)
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultHost
	}
	endpoint, err := url.Parse(host)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return "", "", fmt.Errorf("Galactus host must be an http(s)://host[:port] URL")
	}
	return strings.TrimRight(host, "/"), code, nil
}
