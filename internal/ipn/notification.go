// Package ipn receives PayPal Instant Payment Notifications, records each one, and keeps guild premium in step with
// the subscriptions they describe. See storage/payments.sql for the tables it writes.
package ipn

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // payment_date is in PayPal's Pacific time, and the image may not ship zoneinfo

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"golang.org/x/text/encoding/htmlindex"
)

// Notification is a parsed IPN body with every value decoded to UTF-8.
type Notification struct {
	values url.Values
}

// Parse decodes a form-encoded IPN body. PayPal percent-encodes the bytes of the charset named in the body's charset
// field (windows-1252 unless the account is set to UTF-8), so values are converted from it; anything that still is not
// valid UTF-8 is replaced rather than stored as mojibake.
func Parse(body []byte) (*Notification, error) {
	raw, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("empty notification")
	}
	decode := func(s string) string { return s }
	if enc, err := htmlindex.Get(raw.Get("charset")); err == nil {
		dec := enc.NewDecoder()
		decode = func(s string) string {
			if out, err := dec.String(s); err == nil {
				return out
			}
			return s
		}
	}
	values := make(url.Values, len(raw))
	for k, vs := range raw {
		for _, v := range vs {
			values[k] = append(values[k], strings.ToValidUTF8(decode(v), "�"))
		}
	}
	return &Notification{values: values}, nil
}

// Get returns the first value of an IPN variable, or "".
func (n *Notification) Get(key string) string { return n.values.Get(key) }

// Test reports whether the notification came from the PayPal sandbox.
func (n *Notification) Test() bool { return n.Get("test_ipn") == "1" }

// Kind is the txn_type, or for notifications without one (refunds and reversals) the payment_status.
func (n *Notification) Kind() string {
	if t := n.Get("txn_type"); t != "" {
		return t
	}
	return n.Get("payment_status")
}

// EventKey identifies the notification across PayPal's retries. ipn_track_id is the same on every delivery of one
// notification; kind and txn_id are added so two notifications can never share a key even if PayPal reuses a track
// ID. Without a track ID the body's hash stands in.
func (n *Notification) EventKey(body []byte) string {
	if track := n.Get("ipn_track_id"); track != "" {
		if key := track + ":" + n.Kind() + ":" + n.Get("txn_id"); len(key) <= 64 {
			return key
		}
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// GuildID is the Discord server named in custom, which the premium page fills in when a server is selected.
func (n *Notification) GuildID() (uint64, bool) {
	c := strings.TrimSpace(n.Get("custom"))
	if len(c) < 17 || len(c) > 20 {
		return 0, false
	}
	id, err := strconv.ParseUint(c, 10, 64)
	return id, err == nil
}

// Tier is the premium tier paid for, from the item name or failing that the price, or FreeTier when it is not a
// premium purchase (a donation, say). Signups carry the price in mc_amount3 rather than mc_gross.
func (n *Notification) Tier() premium.Tier {
	switch n.Get("item_name") {
	case "AutoMuteUs Bronze":
		return premium.BronzeTier
	case "AutoMuteUs Silver":
		return premium.SilverTier
	case "AutoMuteUs Gold":
		return premium.GoldTier
	}
	amount := n.Get("mc_gross")
	if amount == "" {
		amount = n.Get("mc_amount3")
	}
	switch amount {
	case "1.50", "1.5":
		return premium.BronzeTier
	case "3.50", "3.5":
		return premium.SilverTier
	case "5.50", "5.5":
		return premium.GoldTier
	}
	return premium.FreeTier
}

var pacific = func() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(err)
	}
	return loc
}()

// PaymentTime is when PayPal took the payment, so a notification retried days later still dates premium from the
// payment. It falls back to now when payment_date is missing, unreadable, or in the future.
func (n *Notification) PaymentTime(now time.Time) time.Time {
	date := n.Get("payment_date")
	for _, layout := range []string{"15:04:05 Jan 02, 2006 MST", "15:04:05 Jan 2, 2006 MST"} {
		if t, err := time.ParseInLocation(layout, date, pacific); err == nil {
			if t.After(now) {
				return now
			}
			return t
		}
	}
	return now
}

// ledgerTx is the tx column of the transactions table. Field names match what the old listener stored (the
// github.com/ammario/paypal-ipn Notification, misspellings included) so old and new rows read alike; SubscrID and
// IpnTrackID are new.
type ledgerTx struct {
	TxnType          string
	TxnID            string
	Business         string
	Custom           string
	ParentTxnID      string
	ReceiptID        string
	RecieverEmail    string
	RecieverID       string
	Resend           bool
	ResidenceCountry string
	TestIPN          bool
	ItemName         string
	ItemNumber       string

	AddressCountry     string
	AddressCity        string
	AddressCountryCode string
	AddressName        string
	AddressState       string
	AddressStatus      string
	AddressStreet      string
	AddressZip         string

	ContactPhone      string
	FirstName         string
	LastName          string
	PayerBusinessName string
	PayerEmail        string
	PayerID           string
	PayerStatus       string

	AuthAmount string
	AuthExpire string
	AuthIfD    string
	AuthStatus string
	Invoice    string

	Currency string
	Fee      float64
	Gross    float64

	PaymentDate   struct{ Time *time.Time }
	PaymentStatus string
	PaymentType   string
	PendingReason string
	ReasonCode    string
	Memo          string

	SubscrID   string
	IpnTrackID string
}

// Gross is mc_gross as a number, 0 when absent.
func (n *Notification) Gross() float64 {
	g, _ := strconv.ParseFloat(n.Get("mc_gross"), 64)
	return g
}

func (n *Notification) ledgerJSON(paid time.Time) ([]byte, error) {
	fee, _ := strconv.ParseFloat(n.Get("mc_fee"), 64)
	tx := ledgerTx{
		TxnType: n.Get("txn_type"), TxnID: n.Get("txn_id"), Business: n.Get("business"), Custom: n.Get("custom"),
		ParentTxnID: n.Get("parent_txn_id"), ReceiptID: n.Get("receipt_id"), RecieverEmail: n.Get("receiver_email"),
		RecieverID: n.Get("receiver_id"), Resend: n.Get("resend") == "true", ResidenceCountry: n.Get("residence_country"),
		TestIPN: n.Test(), ItemName: n.Get("item_name"), ItemNumber: n.Get("item_number"),

		AddressCountry: n.Get("address_country"), AddressCity: n.Get("address_city"),
		AddressCountryCode: n.Get("address_country_code"), AddressName: n.Get("address_name"),
		AddressState: n.Get("address_state"), AddressStatus: n.Get("address_status"),
		AddressStreet: n.Get("address_street"), AddressZip: n.Get("address_zip"),

		ContactPhone: n.Get("contact_phone"), FirstName: n.Get("first_name"), LastName: n.Get("last_name"),
		PayerBusinessName: n.Get("payer_business_name"), PayerEmail: n.Get("payer_email"), PayerID: n.Get("payer_id"),
		PayerStatus: n.Get("payer_status"),

		AuthAmount: n.Get("auth_amount"), AuthExpire: n.Get("auth_exp"), AuthIfD: n.Get("auth_id"),
		AuthStatus: n.Get("auth_status"), Invoice: n.Get("invoice"),

		Currency: n.Get("mc_currency"), Fee: fee, Gross: n.Gross(),

		PaymentStatus: n.Get("payment_status"), PaymentType: n.Get("payment_type"),
		PendingReason: n.Get("pending_reason"), ReasonCode: n.Get("reason_code"), Memo: n.Get("memo"),

		SubscrID: n.Get("subscr_id"), IpnTrackID: n.Get("ipn_track_id"),
	}
	tx.PaymentDate.Time = &paid
	return json.Marshal(tx)
}
