// migrate-guild-settings moves any guild settings still stored in Redis into
// Postgres. The bot does this on its own the first time each guild is read;
// this tool covers guilds that are never read. It is safe to run while the
// bot is running. See storage/GUILD_SETTINGS_MIGRATION.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", false, "only validate the Redis records; write nothing")
	timeout := flag.Duration("timeout", 30*time.Minute, "maximum run time")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return errors.New("REDIS_ADDR is required")
	}
	r := redis.NewClient(&redis.Options{Addr: addr, Username: os.Getenv("REDIS_USER"), Password: os.Getenv("REDIS_PASS")})
	defer r.Close()

	var store *storage.StorageInterface
	if !*dryRun {
		pAddr, pUser, pPass := os.Getenv("POSTGRES_ADDR"), os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASS")
		if pAddr == "" || pUser == "" || pPass == "" {
			return errors.New("POSTGRES_ADDR, POSTGRES_USER and POSTGRES_PASS are required")
		}
		pool, err := pgxpool.Connect(ctx, pgstorage.ConstructPsqlConnectURL(pAddr, pUser, pPass))
		if err != nil {
			return err
		}
		defer pool.Close()
		if err := storage.ApplyGuildSettingsSchema(ctx, pool); err != nil {
			return err
		}
		store = storage.NewPostgresStorage(pool, r)
	}

	report, err := storage.SweepLegacySettings(ctx, r, store, *dryRun)
	if encodeErr := json.NewEncoder(os.Stdout).Encode(report); encodeErr != nil {
		return encodeErr
	}
	return err
}
