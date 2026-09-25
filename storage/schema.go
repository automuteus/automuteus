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

//go:embed payments.sql
var paymentsSchema string

// ApplyPaymentsSchema creates the payment tables cmd/ipn writes. Neither the bot nor the API calls it: the official
// database gets payments.sql applied by hand, since the listener's role has no DDL rights. It exists for tests and
// fresh databases.
func ApplyPaymentsSchema(ctx context.Context, db settingsDB) error {
	if _, err := db.Exec(ctx, "BEGIN; SELECT pg_advisory_xact_lock(754810024);\n"+paymentsSchema+";\nCOMMIT;"); err != nil {
		return fmt.Errorf("apply payments schema: %w", err)
	}
	return nil
}
