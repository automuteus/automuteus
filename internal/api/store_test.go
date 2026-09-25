package api

import (
	"context"
	"errors"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/locale"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/pashagolub/pgxmock"
)

const testGuildID = "123456789012345678"

// newSettingsStore wires a DataStore to a mocked Postgres pool and no Redis. Every expectation must be met and
// nothing unexpected may be executed, so a test with no expectations proves that no write happened.
func newSettingsStore(t *testing.T) (*DataStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		mock.Close()
	})
	return &DataStore{settings: storage.NewPostgresStorage(mock)}, mock
}

func TestDataStoreSetSettings_WritesValidDocument(t *testing.T) {
	store, mock := newSettingsStore(t)
	sett := settings.MakeGuildSettings()
	sett.SetLanguage("de")
	sett.SetLeaderboardSize(7)
	mock.ExpectExec(`INSERT INTO guild_settings .* ON CONFLICT \(guild_hash\) DO NOTHING`).
		WithArgs(pgxmock.AnyArg(), sett.AdminUserIDs, sett.PermissionRoleIDs, "de", nil, sett.MapVersion, nil,
			sett.DeleteGameSummaryMinutes, sett.UnmuteDeadDuringTasks, sett.AutoRefresh, sett.MatchSummaryChannelID,
			sett.LeaderboardMention, 7, sett.LeaderboardMin, sett.MuteSpectator, sett.DisplayRoomCode).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := store.SetSettings(context.Background(), testGuildID, sett, storage.NoSettingsRow); err != nil {
		t.Fatal(err)
	}
}

// The store validates against the embedded language set, so any language shipped under locales/ is accepted.
func TestDataStoreSetSettings_AcceptsEveryEmbeddedLanguage(t *testing.T) {
	store, mock := newSettingsStore(t)
	for lang := range locale.GetLanguages() {
		sett := settings.MakeGuildSettings()
		sett.SetLanguage(lang)
		mock.ExpectExec(`INSERT INTO guild_settings`).WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), lang, pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		if err := store.SetSettings(context.Background(), testGuildID, sett, storage.NoSettingsRow); err != nil {
			t.Errorf("language %q: %v", lang, err)
		}
	}
}

// Whatever the handler did, an invalid document must never reach Postgres, and the error must be recognisable as
// a validation failure rather than a storage failure.
func TestDataStoreSetSettings_RefusesInvalidDocumentWithoutWriting(t *testing.T) {
	cases := map[string]func(*settings.GuildSettings){
		"unknown language":     func(s *settings.GuildSettings) { s.SetLanguage("tlh") },
		"leaderboard too big":  func(s *settings.GuildSettings) { s.SetLeaderboardSize(settings.MaxLeaderboardSize + 1) },
		"bad admin id":         func(s *settings.GuildSettings) { s.SetAdminUserIDs([]string{"not-an-id"}) },
		"bad room code option": func(s *settings.GuildSettings) { s.SetDisplayRoomCode("sometimes") },
		"missing delay table":  func(s *settings.GuildSettings) { s.Delays.Delays = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			store, _ := newSettingsStore(t)
			sett := settings.MakeGuildSettings()
			mutate(sett)
			err := store.SetSettings(context.Background(), testGuildID, sett, storage.NoSettingsRow)
			var verrs settings.ValidationErrors
			if !errors.As(err, &verrs) {
				t.Fatalf("want settings.ValidationErrors, got %T: %v", err, err)
			}
		})
	}
}

func TestDataStoreSetSettings_RefusesNilDocument(t *testing.T) {
	store, _ := newSettingsStore(t)
	if err := store.SetSettings(context.Background(), testGuildID, nil, storage.NoSettingsRow); err == nil {
		t.Fatal("nil settings were accepted")
	}
}

func TestDataStoreSetSettings_RefusesBadGuildID(t *testing.T) {
	for _, id := range []string{"", "abc", "123", "<@123456789012345678>"} {
		store, _ := newSettingsStore(t)
		if err := store.SetSettings(context.Background(), id, settings.MakeGuildSettings(), storage.NoSettingsRow); err == nil {
			t.Errorf("guild ID %q was accepted", id)
		}
	}
}

func TestDataStoreSetSettings_HonoursCancelledContext(t *testing.T) {
	store, _ := newSettingsStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.SetSettings(ctx, testGuildID, settings.MakeGuildSettings(), storage.NoSettingsRow); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestDataStoreSetSettings_ReportsStorageFailure(t *testing.T) {
	store, mock := newSettingsStore(t)
	boom := errors.New("connection reset")
	mock.ExpectExec(`INSERT INTO guild_settings`).WithArgs(
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(boom)
	err := store.SetSettings(context.Background(), testGuildID, settings.MakeGuildSettings(), storage.NoSettingsRow)
	if !errors.Is(err, boom) {
		t.Fatalf("want storage error, got %v", err)
	}
	var verrs settings.ValidationErrors
	if errors.As(err, &verrs) {
		t.Fatal("a storage failure must not look like a validation failure")
	}
}

// With a known row version the write is a conditional UPDATE, and a lost race surfaces as ErrSettingsConflict.
func TestDataStoreSetSettings_ConditionalUpdate(t *testing.T) {
	store, mock := newSettingsStore(t)
	sett := settings.MakeGuildSettings()
	sett.SetLeaderboardSize(7)
	mock.ExpectExec(`UPDATE guild_settings SET .* WHERE guild_hash = \$1 AND version = \$17`).
		WithArgs(pgxmock.AnyArg(), sett.AdminUserIDs, sett.PermissionRoleIDs, sett.Language, nil, sett.MapVersion, nil,
			sett.DeleteGameSummaryMinutes, sett.UnmuteDeadDuringTasks, sett.AutoRefresh, sett.MatchSummaryChannelID,
			sett.LeaderboardMention, 7, sett.LeaderboardMin, sett.MuteSpectator, sett.DisplayRoomCode, int64(4)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := store.SetSettings(context.Background(), testGuildID, sett, 4); err != nil {
		t.Fatal(err)
	}
}

func TestDataStoreSetSettings_ConflictPassesThrough(t *testing.T) {
	store, mock := newSettingsStore(t)
	mock.ExpectExec(`UPDATE guild_settings SET`).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	err := store.SetSettings(context.Background(), testGuildID, settings.MakeGuildSettings(), 4)
	if !errors.Is(err, storage.ErrSettingsConflict) {
		t.Fatalf("want ErrSettingsConflict, got %v", err)
	}
	var verrs settings.ValidationErrors
	if errors.As(err, &verrs) {
		t.Fatal("a conflict must not look like a validation failure")
	}
}

// Validation still runs first: an invalid document is refused before any conditional write is attempted.
func TestDataStoreSetSettings_ValidatesBeforeConditionalWrite(t *testing.T) {
	store, _ := newSettingsStore(t)
	sett := settings.MakeGuildSettings()
	sett.SetLeaderboardSize(99)
	err := store.SetSettings(context.Background(), testGuildID, sett, 4)
	var verrs settings.ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("want ValidationErrors, got %v", err)
	}
}
