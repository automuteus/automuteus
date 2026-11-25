package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestGuildPlayedSince(t *testing.T) {
	since := time.Unix(1700000000, 0)
	query := "^SELECT EXISTS \\(SELECT 1 FROM games WHERE guild_id = (.+) AND start_time >= (.+)\\)$"
	for _, tc := range []struct {
		name    string
		guild   string
		played  bool
		dbErr   error
		wantErr bool
	}{
		{name: "played", guild: "123", played: true},
		{name: "not played", guild: "123"},
		{name: "database error", guild: "123", dbErr: errors.New("database unavailable"), wantErr: true},
		{name: "bad guild ID", guild: "not a guild", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewConn()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close(context.Background())
			if tc.guild == "123" {
				expect := mock.ExpectQuery(query).WithArgs(uint64(123), since.Unix())
				if tc.dbErr != nil {
					expect.WillReturnError(tc.dbErr)
				} else {
					expect.WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(tc.played))
				}
			}
			played, err := guildPlayedSince(context.Background(), mock, tc.guild, since)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
			if played != tc.played {
				t.Fatalf("played = %v, want %v", played, tc.played)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
