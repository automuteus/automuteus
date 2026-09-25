package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/gin-gonic/gin"
)

func TestGuildPremiumSubscription(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const path = "/guild/premium?guildID=123456789012345678"
	active := &SubscriptionStatus{Status: "cancelled", EndsAt: 1793000000, Inherited: true}
	for _, tc := range []struct {
		name  string
		store *fakeStore
		code  int
		want  string
	}{
		{"active premium with a subscription", &fakeStore{premium: &premium.PremiumRecord{Tier: premium.GoldTier, Days: 12}, subscription: active},
			200, `"subscription":{"status":"cancelled","endsAt":1793000000,"inherited":true}`},
		{"active premium without one", &fakeStore{premium: &premium.PremiumRecord{Tier: premium.GoldTier, Days: 12}},
			200, `{"tier":3,"days":12}`},
		// A lapsed subscription adds nothing to an expired tier, so it is not looked up at all.
		{"expired premium", &fakeStore{premium: &premium.PremiumRecord{Tier: premium.GoldTier, Days: 0}, subscriptionErr: errors.New("unreachable")},
			200, `{"tier":3,"days":0}`},
		{"free", &fakeStore{premium: &premium.PremiumRecord{Tier: premium.FreeTier, Days: premium.NoExpiryCode}, subscription: active},
			200, `{"tier":0,"days":-9999}`},
		{"subscription lookup fails", &fakeStore{premium: &premium.PremiumRecord{Tier: premium.GoldTier, Days: 12}, subscriptionErr: errors.New("db down")},
			500, `unable to read subscription`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter(Config{Version: "test", AdminPassword: "test-password"}, tc.store)
			w := request(t, r, path, true)
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("got %d %s, want %d containing %s", w.Code, w.Body, tc.code, tc.want)
			}
			if w.Code == 200 && !json.Valid(w.Body.Bytes()) {
				t.Fatal("invalid JSON")
			}
		})
	}
}
