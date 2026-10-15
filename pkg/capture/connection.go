package capture

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Capture kinds a capture client reports in the "kind" query parameter of its socket URL, with its version in
// "version". Capture Apps older than this report neither.
const (
	KindApp = "capture-app"
	KindMod = "capture-mod"
)

// maxIdentityLength bounds what a capture client can make the bot store about it.
const maxIdentityLength = 32

// ConnectionStatus is the payload of a connection job: whether a capture client is connected for a game, and which
// one. Kind and Version are empty when the capture client didn't report them.
type ConnectionStatus struct {
	Connected bool   `json:"connected"`
	Kind      string `json:"kind,omitempty"`
	Version   string `json:"version,omitempty"`
}

// IdentityFromQuery reads the kind and version a capture client reports in its socket URL's query, dropping values
// too long to be honest.
func IdentityFromQuery(query url.Values) (kind, version string) {
	return identityValue(query.Get("kind")), identityValue(query.Get("version"))
}

func identityValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > maxIdentityLength {
		return ""
	}
	return v
}

func (c ConnectionStatus) Payload() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// ParseConnectionStatus reads a connection job's payload. It also accepts the bare "true" and "false" Galactus sent
// before connection payloads were JSON.
func ParseConnectionStatus(payload string) (ConnectionStatus, error) {
	switch payload {
	case "true":
		return ConnectionStatus{Connected: true}, nil
	case "false":
		return ConnectionStatus{}, nil
	}
	var c ConnectionStatus
	err := json.Unmarshal([]byte(payload), &c)
	return c, err
}
