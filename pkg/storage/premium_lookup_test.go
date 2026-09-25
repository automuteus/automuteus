package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/pashagolub/pgxmock"
)

func TestCheckedGuildPremiumStatus(t *testing.T) {
	for _, scenario := range []string{"active", "expired", "transferred", "inherited", "missing", "missing_parent", "database_error", "parent_error", "cycle"} {
		t.Run(scenario, func(t *testing.T) {
			mock, err := pgxmock.NewConn()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close(context.Background())
			columns := []string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}
			now := int32(time.Now().Unix())
			old := int32(time.Now().Add(-32 * 24 * time.Hour).Unix())
			origin, dest := uint64(123), uint64(456)
			query := "^SELECT (.+) FROM guilds WHERE guild_id = (.+)$"
			wantTier, wantExpired, wantError := premium.GoldTier, false, false
			switch scenario {
			case "active":
				mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns).AddRow(origin, "guild", int16(premium.GoldTier), &now, nil, nil))
			case "expired":
				mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns).AddRow(origin, "guild", int16(premium.GoldTier), &old, nil, nil))
				wantExpired = true
			case "transferred":
				mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns).AddRow(origin, "guild", int16(premium.GoldTier), &now, &dest, nil))
				wantTier, wantExpired = premium.FreeTier, true
			case "inherited", "missing_parent", "parent_error":
				mock.ExpectQuery(query).WithArgs(dest).WillReturnRows(pgxmock.NewRows(columns).AddRow(dest, "dest", int16(premium.FreeTier), nil, nil, &origin))
				switch scenario {
				case "inherited":
					mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns).AddRow(origin, "origin", int16(premium.GoldTier), &now, &dest, nil))
				case "missing_parent":
					mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns))
					wantError = true
				case "parent_error":
					mock.ExpectQuery(query).WithArgs(origin).WillReturnError(errors.New("database unavailable"))
					wantError = true
				}
			case "missing":
				mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns))
				wantTier, wantExpired = premium.FreeTier, true
			case "database_error":
				mock.ExpectQuery(query).WithArgs(origin).WillReturnError(errors.New("database unavailable"))
				wantError = true
			case "cycle":
				for i := 0; i < 4; i++ {
					mock.ExpectQuery(query).WithArgs(origin).WillReturnRows(pgxmock.NewRows(columns).AddRow(origin, "cycle", int16(premium.FreeTier), nil, nil, &origin))
				}
				wantError = true
			}
			guild := "123"
			if scenario == "inherited" || scenario == "missing_parent" || scenario == "parent_error" {
				guild = "456"
			}
			tier, days, err := checkedGuildPremiumStatus(context.Background(), mock, guild, 0)
			if (err != nil) != wantError {
				t.Fatalf("error = %v, wantError = %v", err, wantError)
			}
			if !wantError && (tier != wantTier || premium.IsExpired(tier, days) != wantExpired) {
				t.Fatalf("tier=%v days=%d; want tier=%v expired=%v", tier, days, wantTier, wantExpired)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetGuildPremiumStatusSelfHosted(t *testing.T) {
	// Self-hosted workers are not subject to hosted subscription expiry, and need no DB query.
	s := &PsqlInterface{}
	tier, days, err := s.GetGuildPremiumStatus(context.Background(), false, "123")
	if err != nil || tier != premium.SelfHostTier || days != premium.NoExpiryCode {
		t.Fatalf("%v %d %v", tier, days, err)
	}
}
