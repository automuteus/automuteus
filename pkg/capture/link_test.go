package capture

import "testing"

func TestFormCaptureURL(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"http://localhost:8123", "aucapture://localhost:8123/ABCDEFGH?insecure"},
		{"http://example.com", "aucapture://example.com:80/ABCDEFGH?insecure"},
		{"https://example.com", "aucapture://example.com:443/ABCDEFGH"},
		{"https://example.com:8443/", "aucapture://example.com:8443/ABCDEFGH"},
	} {
		link, apiLink, _ := FormCaptureURL(tc.host, "https://api.example.com", "ABCDEFGH")
		if link != tc.want || apiLink != "https://api.example.com/open/link?connectCode=ABCDEFGH" {
			t.Errorf("%s: %s %s", tc.host, link, apiLink)
		}
	}
}
