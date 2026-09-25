package ipn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	LiveVerifyURL    = "https://ipnpb.paypal.com/cgi-bin/webscr"
	SandboxVerifyURL = "https://ipnpb.sandbox.paypal.com/cgi-bin/webscr"
)

// Verifier asks the payment provider whether a notification body is genuine. An error means the answer is unknown
// (network trouble, an outage), and the notification should be retried rather than dropped.
type Verifier interface {
	Verify(ctx context.Context, body []byte) (bool, error)
}

// PayPalVerifier posts the notification back to PayPal, byte for byte, as its IPN protocol requires.
type PayPalVerifier struct {
	URL    string
	Client *http.Client
}

func NewPayPalVerifier(sandbox bool) *PayPalVerifier {
	url := LiveVerifyURL
	if sandbox {
		url = SandboxVerifyURL
	}
	return &PayPalVerifier{URL: url, Client: &http.Client{Timeout: 15 * time.Second}}
}

func (v *PayPalVerifier) Verify(ctx context.Context, body []byte) (bool, error) {
	payload := append([]byte("cmd=_notify-validate&"), body...)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.URL, bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// PayPal rejects verification requests without a User-Agent.
	req.Header.Set("User-Agent", "AutoMuteUs-IPN")
	resp, err := v.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf("verify: %w", err)
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return false, fmt.Errorf("verify: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("verify: PayPal answered %d", resp.StatusCode)
	}
	switch string(bytes.TrimSpace(answer)) {
	case "VERIFIED":
		return true, nil
	case "INVALID":
		return false, nil
	}
	return false, fmt.Errorf("verify: unexpected answer %q", answer)
}
