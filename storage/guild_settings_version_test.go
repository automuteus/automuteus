package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/jackc/pgx/v4"
	"github.com/pashagolub/pgxmock"
)

// versionPlaceholder is the positional parameter the expected version occupies: after the hash and every column.
var versionPlaceholder = "$" + strconv.Itoa(len(settingsColumns)+2)

func TestUpdateIfVersionMatchesColumns(t *testing.T) {
	for i, column := range settingsColumns {
		if want := fmt.Sprintf("%s = $%d", column, i+2); !strings.Contains(updateIfVersion, want) {
			t.Errorf("update statement missing %q:\n%s", want, updateIfVersion)
		}
	}
	if !strings.HasSuffix(updateIfVersion, "WHERE guild_hash = $1 AND version = "+versionPlaceholder) {
		t.Errorf("update statement must be conditional on the version:\n%s", updateIfVersion)
	}
	if !strings.Contains(updateIfVersion, "version = version + 1") {
		t.Error("update statement must bump the version")
	}
}

func TestLoadVersionReturnsRowVersion(t *testing.T) {
	mock := newMock(t)
	id := "123"
	hash := string(rediskey.HashGuildID(id))
	mock.ExpectQuery("SELECT .* version FROM guild_settings WHERE guild_hash").WithArgs(hash).
		WillReturnRows(pgxmock.NewRows(selectColumns).AddRow(selectRow(defaultRow(), 7)...))
	sett, version, err := NewPostgresStorage(mock, nil).LoadGuildSettingsVersion(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if version != 7 {
		t.Errorf("version = %d, want 7", version)
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), sett)
}

func TestLoadVersionMissingRowIsNoSettingsRow(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT .* FROM guild_settings WHERE guild_hash").WillReturnError(pgx.ErrNoRows)
	sett, version, err := NewPostgresStorage(mock, nil).LoadGuildSettingsVersion(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if version != NoSettingsRow {
		t.Errorf("version = %d, want NoSettingsRow", version)
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), sett)
}

func TestLoadVersionFailureDoesNotReturnSettings(t *testing.T) {
	mock := newMock(t)
	failure := errors.New("database unavailable")
	mock.ExpectQuery("SELECT .* FROM guild_settings WHERE guild_hash").WillReturnError(failure)
	sett, _, err := NewPostgresStorage(mock, nil).LoadGuildSettingsVersion(context.Background(), "123")
	if !errors.Is(err, failure) || sett != nil {
		t.Fatalf("got %v, %v", sett, err)
	}
}

// A row must never report a version below 1: that would let a conditional write treat it as absent.
func TestLoadVersionRejectsImpossibleVersion(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT .* FROM guild_settings WHERE guild_hash").
		WillReturnRows(pgxmock.NewRows(selectColumns).AddRow(selectRow(defaultRow(), 0)...))
	sett, _, err := NewPostgresStorage(mock, nil).LoadGuildSettingsVersion(context.Background(), "123")
	if err == nil || sett != nil {
		t.Fatalf("version 0 was accepted: %v, %v", sett, err)
	}
}

// The settings and their version come from one query, so there is no window in which another writer can change
// the row between the two: exactly one SELECT is issued.
func TestLoadVersionIsASingleQuery(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT .* FROM guild_settings WHERE guild_hash").
		WillReturnRows(pgxmock.NewRows(selectColumns).AddRow(selectRow(defaultRow(), 3)...))
	if _, version, err := NewPostgresStorage(mock, nil).LoadGuildSettingsVersion(context.Background(), "123"); err != nil || version != 3 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	// newMock's cleanup fails the test if a second query had been expected but not issued, and pgxmock fails
	// any query that was not expected; together they pin the count to one.
}

func TestSetIfVersionCreatesMissingRow(t *testing.T) {
	mock := newMock(t)
	id := "123"
	args := append([]interface{}{string(rediskey.HashGuildID(id))}, defaultRow()...)
	mock.ExpectExec(`INSERT INTO guild_settings .* ON CONFLICT \(guild_hash\) DO NOTHING`).WithArgs(args...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := NewPostgresStorage(mock, nil).SetGuildSettingsIfVersion(context.Background(), id, settings.MakeGuildSettings(), NoSettingsRow); err != nil {
		t.Fatal(err)
	}
}

// A row that appeared between the read and the write is a conflict, not an overwrite.
func TestSetIfVersionCreateConflictsWhenRowAppeared(t *testing.T) {
	mock := newMock(t)
	mock.ExpectExec(`INSERT INTO guild_settings .* DO NOTHING`).WillReturnResult(pgxmock.NewResult("INSERT", 0))
	err := NewPostgresStorage(mock, nil).SetGuildSettingsIfVersion(context.Background(), "123", settings.MakeGuildSettings(), NoSettingsRow)
	if !errors.Is(err, ErrSettingsConflict) {
		t.Fatalf("want ErrSettingsConflict, got %v", err)
	}
}

func TestSetIfVersionUpdatesMatchingRow(t *testing.T) {
	mock := newMock(t)
	id := "123"
	sett := loadFixture(t)
	args := []interface{}{string(rediskey.HashGuildID(id)), sett.AdminUserIDs, sett.PermissionRoleIDs, sett.Language,
		mustMarshal(t, sett.VoiceRules), sett.MapVersion, mustMarshal(t, sett.Delays), sett.DeleteGameSummaryMinutes,
		sett.UnmuteDeadDuringTasks, sett.AutoRefresh, sett.MatchSummaryChannelID, sett.LeaderboardMention,
		sett.LeaderboardSize, sett.LeaderboardMin, sett.MuteSpectator, sett.DisplayRoomCode, int64(3)}
	mock.ExpectExec(`UPDATE guild_settings SET .* version = version \+ 1.* WHERE guild_hash = \$1 AND version = \` + versionPlaceholder).
		WithArgs(args...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := NewPostgresStorage(mock, nil).SetGuildSettingsIfVersion(context.Background(), id, sett, 3); err != nil {
		t.Fatal(err)
	}
}

func TestSetIfVersionUpdateConflictsWhenVersionMoved(t *testing.T) {
	mock := newMock(t)
	mock.ExpectExec(`UPDATE guild_settings SET`).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	err := NewPostgresStorage(mock, nil).SetGuildSettingsIfVersion(context.Background(), "123", settings.MakeGuildSettings(), 3)
	if !errors.Is(err, ErrSettingsConflict) {
		t.Fatalf("want ErrSettingsConflict, got %v", err)
	}
}

func TestSetIfVersionReportsStorageFailure(t *testing.T) {
	mock := newMock(t)
	failure := errors.New("connection reset")
	mock.ExpectExec(`UPDATE guild_settings SET`).WillReturnError(failure)
	err := NewPostgresStorage(mock, nil).SetGuildSettingsIfVersion(context.Background(), "123", settings.MakeGuildSettings(), 3)
	if !errors.Is(err, failure) || errors.Is(err, ErrSettingsConflict) {
		t.Fatalf("want the storage error, got %v", err)
	}
}

func TestSetIfVersionRejectsBadInput(t *testing.T) {
	store := NewPostgresStorage(newMock(t), nil)
	if err := store.SetGuildSettingsIfVersion(context.Background(), "123", nil, 1); err == nil {
		t.Error("nil settings were accepted")
	}
	if err := store.SetGuildSettingsIfVersion(context.Background(), "123", settings.MakeGuildSettings(), -1); err == nil {
		t.Error("negative version was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.SetGuildSettingsIfVersion(ctx, "123", settings.MakeGuildSettings(), 1); !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

// The bot's unconditional upsert must bump the version too, so an API client holding an old ETag sees the change.
func TestUpsertBumpsVersion(t *testing.T) {
	if !strings.Contains(upsertSettings, "version = guild_settings.version + 1") {
		t.Errorf("upsert must bump the version:\n%s", upsertSettings)
	}
}
