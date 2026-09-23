package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/jackc/pgx/v4"
	"github.com/pashagolub/pgxmock"
)

const fixturePath = "../pkg/settings/testdata/guild_settings_v8.json"

func loadFixture(t *testing.T) *settings.GuildSettings {
	t.Helper()
	blob, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var sett settings.GuildSettings
	if err := json.Unmarshal(blob, &sett); err != nil {
		t.Fatal(err)
	}
	return &sett
}

func assertSettingsEqual(t *testing.T, want, got *settings.GuildSettings) {
	t.Helper()
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Fatalf("settings differ\nwant %s\n got %s", a, b)
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return blob
}

func newMock(t *testing.T) pgxmock.PgxPoolIface {
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
	return mock
}

// defaultRow is what a row written from MakeGuildSettings looks like: both
// documents collapse to NULL. These are the insert/update arguments, without the version.
func defaultRow() []interface{} {
	return []interface{}{[]string{}, []string{}, "en", nil, "simple", nil, 0, false, false, "", true, 3, 3, false, "always"}
}

// selectRow is what a SELECT returns: the settings columns plus the row version.
func selectRow(values []interface{}, version int64) []interface{} {
	return append(append([]interface{}{}, values...), version)
}

// The select must return the settings columns in insert order followed only by the version, since loadPostgres
// scans positionally.
func TestSelectColumnsAreSettingsColumnsPlusVersion(t *testing.T) {
	if len(selectColumns) != len(settingsColumns)+1 || selectColumns[len(selectColumns)-1] != "version" {
		t.Fatalf("selectColumns = %v", selectColumns)
	}
	for i, column := range settingsColumns {
		if selectColumns[i] != column {
			t.Errorf("selectColumns[%d] = %s, want %s", i, selectColumns[i], column)
		}
	}
}

func TestSettingsArgsMatchColumns(t *testing.T) {
	args, err := settingsArgs("hash", settings.MakeGuildSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != len(settingsColumns)+1 {
		t.Fatalf("%d args for %d columns plus hash", len(args), len(settingsColumns))
	}
}

func TestLoadResolvesNullDocumentsToDefaults(t *testing.T) {
	mock := newMock(t)
	id := "123"
	mock.ExpectQuery("SELECT .* FROM guild_settings WHERE guild_hash").WithArgs(string(rediskey.HashGuildID(id))).
		WillReturnRows(pgxmock.NewRows(selectColumns).AddRow(selectRow(defaultRow(), 1)...))
	got, err := NewPostgresStorage(mock, nil).LoadGuildSettings(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), got)
}

func TestLoadDecodesStoredDocuments(t *testing.T) {
	mock := newMock(t)
	want := loadFixture(t)
	mock.ExpectQuery("SELECT .* FROM guild_settings").WillReturnRows(pgxmock.NewRows(selectColumns).AddRow(
		want.AdminUserIDs, want.PermissionRoleIDs, want.Language, mustMarshal(t, want.VoiceRules), want.MapVersion,
		mustMarshal(t, want.Delays), want.DeleteGameSummaryMinutes, want.UnmuteDeadDuringTasks, want.AutoRefresh,
		want.MatchSummaryChannelID, want.LeaderboardMention, want.LeaderboardSize, want.LeaderboardMin,
		want.MuteSpectator, want.DisplayRoomCode, int64(9)))
	got, err := NewPostgresStorage(mock, nil).LoadGuildSettings(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	assertSettingsEqual(t, want, got)
}

func TestLoadMissingGuildWithoutLegacyReturnsDefaultsWithoutWriting(t *testing.T) {
	mock := newMock(t)
	mock.ExpectQuery("SELECT .* FROM guild_settings").WillReturnError(pgx.ErrNoRows)
	got, err := NewPostgresStorage(mock, nil).LoadGuildSettings(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), got)
}

func TestLoadFailureDoesNotReturnDefaults(t *testing.T) {
	mock := newMock(t)
	failure := errors.New("database unavailable")
	mock.ExpectQuery("SELECT .* FROM guild_settings").WillReturnError(failure)
	got, err := NewPostgresStorage(mock, nil).LoadGuildSettings(context.Background(), "123")
	if !errors.Is(err, failure) || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestLoadMalformedDocumentFails(t *testing.T) {
	mock := newMock(t)
	row := selectRow(defaultRow(), 1)
	row[3] = []byte(`[1]`)
	mock.ExpectQuery("SELECT .* FROM guild_settings").WillReturnRows(pgxmock.NewRows(selectColumns).AddRow(row...))
	got, err := NewPostgresStorage(mock, nil).LoadGuildSettings(context.Background(), "123")
	if err == nil || got != nil {
		t.Fatalf("malformed document did not fail: %v, %v", got, err)
	}
}

func TestSetStoresDefaultDocumentsAsNull(t *testing.T) {
	mock := newMock(t)
	id := "123"
	args := append([]interface{}{string(rediskey.HashGuildID(id))}, defaultRow()...)
	mock.ExpectExec(`INSERT INTO guild_settings .* ON CONFLICT \(guild_hash\) DO UPDATE SET`).WithArgs(args...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := NewPostgresStorage(mock, nil).SetGuildSettings(id, settings.MakeGuildSettings()); err != nil {
		t.Fatal(err)
	}
}

func TestSetStoresCustomDocuments(t *testing.T) {
	mock := newMock(t)
	sett := loadFixture(t)
	mock.ExpectExec(`INSERT INTO guild_settings .* DO UPDATE SET`).WithArgs(
		pgxmock.AnyArg(), sett.AdminUserIDs, sett.PermissionRoleIDs, sett.Language, mustMarshal(t, sett.VoiceRules),
		sett.MapVersion, mustMarshal(t, sett.Delays), sett.DeleteGameSummaryMinutes, sett.UnmuteDeadDuringTasks,
		sett.AutoRefresh, sett.MatchSummaryChannelID, sett.LeaderboardMention, sett.LeaderboardSize,
		sett.LeaderboardMin, sett.MuteSpectator, sett.DisplayRoomCode).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := NewPostgresStorage(mock, nil).SetGuildSettings("123", sett); err != nil {
		t.Fatal(err)
	}
}

func TestSetWithContextStoresCustomDocuments(t *testing.T) {
	mock := newMock(t)
	sett := loadFixture(t)
	mock.ExpectExec(`INSERT INTO guild_settings .* DO UPDATE SET`).WithArgs(
		pgxmock.AnyArg(), sett.AdminUserIDs, sett.PermissionRoleIDs, sett.Language, mustMarshal(t, sett.VoiceRules),
		sett.MapVersion, mustMarshal(t, sett.Delays), sett.DeleteGameSummaryMinutes, sett.UnmuteDeadDuringTasks,
		sett.AutoRefresh, sett.MatchSummaryChannelID, sett.LeaderboardMention, sett.LeaderboardSize,
		sett.LeaderboardMin, sett.MuteSpectator, sett.DisplayRoomCode).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := NewPostgresStorage(mock, nil).SetGuildSettingsContext(context.Background(), "123", sett); err != nil {
		t.Fatal(err)
	}
}

// A request that was already cancelled must not reach the database.
func TestSetWithCancelledContextDoesNotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewPostgresStorage(newMock(t), nil).SetGuildSettingsContext(ctx, "123", loadFixture(t))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestSetWithContextNilSettingsFails(t *testing.T) {
	if err := NewPostgresStorage(newMock(t), nil).SetGuildSettingsContext(context.Background(), "123", nil); err == nil {
		t.Fatal("nil settings were accepted")
	}
}

func TestSetNilSettingsFails(t *testing.T) {
	if err := NewPostgresStorage(newMock(t), nil).SetGuildSettings("123", nil); err == nil {
		t.Fatal("nil settings were accepted")
	}
}

func TestDeleteRemovesRow(t *testing.T) {
	mock := newMock(t)
	id := "123"
	mock.ExpectExec("DELETE FROM guild_settings WHERE guild_hash").WithArgs(string(rediskey.HashGuildID(id))).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := NewPostgresStorage(mock, nil).DeleteGuildSettings(id); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeLegacySettings(t *testing.T) {
	for _, blob := range []string{``, `null`, `[]`, `{"language":7}`, `{} {}`} {
		for _, strict := range []bool{false, true} {
			if _, err := decodeLegacySettings([]byte(blob), strict); err == nil {
				t.Errorf("accepted %q (strict=%v)", blob, strict)
			}
		}
	}
	if _, err := decodeLegacySettings([]byte(`{"unknownSetting":true}`), true); err == nil {
		t.Error("strict decode accepted an unknown field")
	}
	if _, err := decodeLegacySettings([]byte(`{"unknownSetting":true}`), false); err != nil {
		t.Errorf("lenient decode rejected an unknown field: %v", err)
	}
	// Legacy records decode into a zero struct: absent fields stay zero.
	for _, blob := range []string{`{}`, `{"language":"en","adminIDs":[]}`, `{"voiceRules":{"MuteRules":{"TASKS":{"alive":false}}}}`} {
		var want settings.GuildSettings
		if err := json.Unmarshal([]byte(blob), &want); err != nil {
			t.Fatal(err)
		}
		got, err := decodeLegacySettings([]byte(blob), true)
		if err != nil {
			t.Fatal(err)
		}
		assertSettingsEqual(t, &want, got)
	}
}
