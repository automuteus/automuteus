package storage

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed postgres.sql
var baseSchema string

// ApplySchemas lets either the bot or API start first on a fresh self-hosted
// database. Both processes serialize base DDL; official deployments continue
// to manage the statistics schema separately.
func ApplySchemas(ctx context.Context, db settingsDB, official bool) error {
	if !official {
		if _, err := db.Exec(ctx, "BEGIN; SELECT pg_advisory_xact_lock(754810024);\n"+baseSchema+";\nCOMMIT;"); err != nil {
			return fmt.Errorf("apply base Postgres schema: %w", err)
		}
	}
	if err := ApplyGuildSettingsSchema(ctx, db); err != nil {
		return fmt.Errorf("apply guild settings schema: %w", err)
	}
	return nil
}
