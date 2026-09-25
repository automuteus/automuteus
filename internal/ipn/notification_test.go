package ipn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
)

func TestParseCharsets(t *testing.T) {
	// "Zoë" in windows-1252, and "山田" in Shift_JIS, as PayPal percent-encodes them.
	for _, tc := range []struct{ body, want string }{
		{"charset=windows-1252&first_name=Zo%EB", "Zoë"},
		{"charset=Shift_JIS&first_name=%8ER%93c", "山田"},
		{"charset=UTF-8&first_name=%E5%B1%B1%E7%94%B0", "山田"},
		{"first_name=Zo%EB", "Zo�"}, // no charset: invalid UTF-8 is replaced, not stored raw
		{"charset=nonsense&first_name=abc", "abc"},
	} {
		n, err := Parse([]byte(tc.body))
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		if got := n.Get("first_name"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.body, got, tc.want)
		}
	}
	if _, err := Parse(nil); err == nil {
		t.Error("parsed an empty body")
	}
	if _, err := Parse([]byte("a=%zz")); err == nil {
		t.Error("parsed a malformed body")
	}
}

func TestTier(t *testing.T) {
	for body, want := range map[string]premium.Tier{
		"item_name=AutoMuteUs+Gold&mc_gross=1.50":   premium.GoldTier, // the name wins over the amount
		"item_name=AutoMuteUs+Silver":               premium.SilverTier,
		"item_name=Donation&mc_gross=1.50":          premium.BronzeTier, // as the old listener did
		"txn_type=subscr_signup&mc_amount3=3.50":    premium.SilverTier,
		"item_name=Donation&mc_gross=5.00":          premium.FreeTier,
		"item_name=AutoMuteUs+Platinum&mc_gross=99": premium.FreeTier,
	} {
		n, _ := Parse([]byte(body))
		if got := n.Tier(); got != want {
			t.Errorf("%s: got %v, want %v", body, got, want)
		}
	}
}

func TestGuildID(t *testing.T) {
	for custom, ok := range map[string]bool{
		"1105475628758749234": true, "  1105475628758749234 ": true, "": false, "123": false,
		"99999999999999999999": false, "abcdefghijklmnopqrs": false, "-105475628758749234": false,
	} {
		n := &Notification{values: map[string][]string{"custom": {custom}}}
		if _, got := n.GuildID(); got != ok {
			t.Errorf("%q: got %t", custom, got)
		}
	}
}

func TestPaymentTime(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	for date, want := range map[string]time.Time{
		"09:15:03 Sep 24, 2026 PDT": time.Date(2026, 9, 24, 16, 15, 3, 0, time.UTC),
		"09:15:03 Jan 4, 2026 PST":  time.Date(2026, 1, 4, 17, 15, 3, 0, time.UTC),
		"09:15:03 Sep 26, 2026 PDT": now, // future
		"yesterday":                 now,
		"":                          now,
	} {
		n := &Notification{values: map[string][]string{"payment_date": {date}}}
		if got := n.PaymentTime(now); !got.Equal(want) {
			t.Errorf("%q: got %v, want %v", date, got, want)
		}
	}
}

func TestEventKey(t *testing.T) {
	a, _ := Parse([]byte("ipn_track_id=abc123&txn_type=subscr_payment&txn_id=9X"))
	b, _ := Parse([]byte("ipn_track_id=abc123&txn_type=subscr_signup"))
	if a.EventKey(nil) == b.EventKey(nil) {
		t.Error("two notifications sharing a track ID got one key")
	}
	if got := a.EventKey(nil); got != "abc123:subscr_payment:9X" {
		t.Errorf("got %q", got)
	}
	c, _ := Parse([]byte("txn_id=9X"))
	if k := c.EventKey([]byte("txn_id=9X")); len(k) != 64 || k != c.EventKey([]byte("txn_id=9X")) {
		t.Errorf("hash fallback %q", k)
	}
}

func TestLedgerJSON(t *testing.T) {
	n, _ := Parse([]byte("txn_type=subscr_payment&txn_id=9X&mc_gross=3.50&mc_fee=0.45&subscr_id=I-1&custom=1105475628758749234&test_ipn=1&resend=true"))
	paid := time.Unix(1790340905, 0).UTC()
	raw, err := n.ledgerJSON(paid)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	json.Unmarshal(raw, &got)
	if got["TxnID"] != "9X" || got["Gross"] != 3.5 || got["Fee"] != 0.45 || got["SubscrID"] != "I-1" ||
		got["TestIPN"] != true || got["Resend"] != true || got["Custom"] != "1105475628758749234" {
		t.Errorf("unexpected ledger %s", raw)
	}
	if date := got["PaymentDate"].(map[string]interface{})["Time"]; date != "2026-09-25T12:55:05Z" {
		t.Errorf("payment date %v", date)
	}
}

func TestReject(t *testing.T) {
	l := &Listener{Receiver: "Shop@Example.com"}
	for body, rejected := range map[string]bool{
		"receiver_email=shop%40example.com&mc_currency=USD":  false,
		"business=SHOP%40example.com":                        false,
		"receiver_email=other%40example.com&mc_currency=USD": true,
		"receiver_email=shop%40example.com&mc_currency=JPY":  true,
		"mc_currency=USD": true,
	} {
		n, _ := Parse([]byte(body))
		if got := l.reject(n) != ""; got != rejected {
			t.Errorf("%s: rejected=%t", body, got)
		}
	}
}

func TestPayPalVerifier(t *testing.T) {
	var answer string
	var status int
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent")
		}
		w.WriteHeader(status)
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	v := &PayPalVerifier{URL: srv.URL, Client: srv.Client()}
	body := []byte("first_name=Zo%EB&charset=windows-1252")
	for _, tc := range []struct {
		status  int
		answer  string
		ok, err bool
	}{
		{200, "VERIFIED", true, false},
		{200, "INVALID", false, false},
		{200, "<html>maintenance</html>", false, true},
		{503, "VERIFIED", false, true},
	} {
		status, answer = tc.status, tc.answer
		ok, err := v.Verify(context.Background(), body)
		if ok != tc.ok || (err != nil) != tc.err {
			t.Errorf("%d %q: ok=%t err=%v", tc.status, tc.answer, ok, err)
		}
	}
	// The body must go back untouched, not re-encoded.
	if !strings.HasSuffix(got, "&first_name=Zo%EB&charset=windows-1252") || !strings.HasPrefix(got, "cmd=_notify-validate&") {
		t.Errorf("posted %q", got)
	}
}
