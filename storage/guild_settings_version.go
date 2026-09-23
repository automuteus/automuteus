package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/jackc/pgconn"
)

// SettingsVersion counts writes to a guild's settings row. It is 1 when the
// row is created and increases by one on every write, including the bot's own
// slash-command writes, so a client holding an old version always notices.
type SettingsVersion int64

// NoSettingsRow is the version of a guild that has no row yet and therefore
// uses the built-in defaults. A conditional write against it succeeds only if
// it creates the row.
const NoSettingsRow SettingsVersion = 0

// ErrSettingsConflict is returned by SetGuildSettingsIfVersion when the row's
// version no longer matches: someone else wrote in between. The caller should
// reload, re-apply its change, and retry.
var ErrSettingsConflict = errors.New("guild settings were changed by another writer")

var updateIfVersion = buildUpdateIfVersion()

// buildUpdateIfVersion produces an UPDATE with the same column/placeholder
// order as the insert ($1 hash, $2.. columns) plus the expected version last.
func buildUpdateIfVersion() string {
	assignments := make([]string, 0, len(settingsColumns))
	for i, column := range settingsColumns {
		assignments = append(assignments, fmt.Sprintf("%s = $%d", column, i+2))
	}
	return "UPDATE guild_settings SET " + strings.Join(assignments, ", ") +
		", version = version + 1, updated_at = now() WHERE guild_hash = $1 AND version = $" + strconv.Itoa(len(settingsColumns)+2)
}

// SetGuildSettingsIfVersion writes sett only if the row is still at expected:
// it creates the row when expected is NoSettingsRow, and otherwise updates the
// row whose version matches, bumping it. Any other state, including a row that
// appeared or changed since the read, is ErrSettingsConflict and nothing is
// written. Like SetGuildSettingsContext it does not validate.
func (s *StorageInterface) SetGuildSettingsIfVersion(ctx context.Context, guildID string, sett *settings.GuildSettings, expected SettingsVersion) error {
	if sett == nil {
		return errors.New("nil guild settings")
	}
	if expected < NoSettingsRow {
		return fmt.Errorf("invalid settings version %d", expected)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, settingsTimeout)
	defer cancel()
	hash := string(rediskey.HashGuildID(guildID))
	args, err := settingsArgs(hash, sett)
	if err != nil {
		return err
	}
	var tag pgconn.CommandTag
	if expected == NoSettingsRow {
		tag, err = s.db.Exec(ctx, insertIfAbsent, args...)
	} else {
		tag, err = s.db.Exec(ctx, updateIfVersion, append(args, int64(expected))...)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrSettingsConflict
	}
	if err := s.deleteLegacy(ctx, hash); err != nil {
		log.Println(err)
	}
	return nil
}
