package capture

import (
	"fmt"
	"regexp"
)

var captureURLPattern = regexp.MustCompile(`^http(?P<secure>s?)://(?P<host>[\w.-]+)(?::(?P<port>\d+))?/?$`)

// FormCaptureURL is shared by Discord commands and the standalone API so both
// launch the capture client with the same host, port and TLS settings.
func FormCaptureURL(hostURL, apiURL, connectCode string) (hyperlink, apiHyperlink, minimalURL string) {
	match := captureURLPattern.FindStringSubmatch(hostURL)
	if match == nil {
		return "Invalid HOST provided (should resemble something like `http://localhost:8123`)", "Invalid API Link", "Invalid HOST provided"
	}
	secure := match[captureURLPattern.SubexpIndex("secure")] == "s"
	host := match[captureURLPattern.SubexpIndex("host")]
	port := match[captureURLPattern.SubexpIndex("port")]
	if port == "" {
		if secure {
			port = "443"
		} else {
			port = "80"
		}
	}
	insecure, protocol := "?insecure", "http://"
	if secure {
		insecure, protocol = "", "https://"
	}
	if apiURL == "" {
		apiURL = "http://localhost"
	}
	return fmt.Sprintf("aucapture://%s:%s/%s%s", host, port, connectCode, insecure),
		fmt.Sprintf("%s/open/link?connectCode=%s", apiURL, connectCode),
		fmt.Sprintf("%s%s:%s", protocol, host, port)
}
