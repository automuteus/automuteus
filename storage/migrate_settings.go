package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-redis/redis/v8"
)

type MigrationReport struct {
	Examined int               `json:"examined"`
	Inserted int               `json:"inserted"`
	Deleted  int               `json:"deleted"`
	Failures map[string]string `json:"failures,omitempty"`
}

var guildHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

const legacySettingsPrefix = "automuteus:settings:guild:"

// SweepLegacySettings moves every remaining Redis settings record into
// Postgres, the same way the bot does on first read. It is safe to run while
// the bot is serving traffic: rows are only inserted, never overwritten, and a
// record whose guild already has a row is simply removed. Records that fail
// validation are reported and left in place. With dryRun, nothing is written.
func SweepLegacySettings(ctx context.Context, client *redis.Client, store *StorageInterface, dryRun bool) (MigrationReport, error) {
	report := MigrationReport{Failures: map[string]string{}}
	if !dryRun && (store == nil || store.db == nil) {
		return report, errors.New("Postgres storage is required")
	}
	seen := make(map[string]bool)
	iterator := client.Scan(ctx, 0, legacySettingsPrefix+"*", 500).Iterator()
	for iterator.Next(ctx) {
		key := iterator.Val()
		if seen[key] {
			continue
		}
		seen[key] = true
		report.Examined++
		inserted, err := sweepKey(ctx, client, store, dryRun, key)
		if err != nil {
			report.Failures[key] = err.Error()
			continue
		}
		if dryRun {
			continue
		}
		report.Deleted++
		if inserted {
			report.Inserted++
		}
	}
	if err := iterator.Err(); err != nil {
		return report, err
	}
	if len(report.Failures) > 0 {
		return report, fmt.Errorf("%d settings records failed; their Redis keys were retained", len(report.Failures))
	}
	return report, nil
}

func sweepKey(ctx context.Context, client *redis.Client, store *StorageInterface, dryRun bool, key string) (bool, error) {
	hash := strings.TrimPrefix(key, legacySettingsPrefix)
	if !guildHashPattern.MatchString(hash) {
		return false, errors.New("invalid guild hash")
	}
	blob, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return false, err
	}
	sett, err := decodeLegacySettings(blob, true)
	if err != nil {
		return false, err
	}
	if dryRun {
		return false, nil
	}
	return importLegacySettings(ctx, store.db, client, key, hash, sett)
}
