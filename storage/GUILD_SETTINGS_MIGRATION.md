# Guild settings in Postgres

Guild settings live in the `guild_settings` table. Redis keeps its other
roles (game state, queues, locks) but no longer stores settings.

`guild_settings.sql` is embedded in the binary and applied on every startup.
It is idempotent and takes an advisory lock, so several shards can start at
once. The startup database role needs permission to create the table.

## Representation

- Rows are keyed by the same SHA-256 guild ID hash the Redis records used, so
  every legacy record can be moved without knowing its guild ID.
- Scalar settings have typed columns. `voice_rules` and `delays` are JSONB and
  are NULL when the guild uses the built-in defaults. The defaults are defined
  in Go (`pkg/game`); the database never resolves them. A guild with no row at
  all uses the defaults, so resetting settings deletes the row.
- Reads never substitute defaults for a failure. Storage or decoding errors are
  returned, and handlers tell the user to retry.
- Writes replace the whole document, so concurrent edits keep the old
  last-writer-wins behavior.

## Upgrading from a Redis-backed version

Nothing needs to happen before starting the new version. The first time a
guild is read and has no Postgres row, the bot reads its Redis record, inserts
it into Postgres (never overwriting a row that already exists), and deletes the
Redis key. A record that fails to insert is left in Redis and retried on the
next read. A record that cannot be decoded is logged, left in Redis, and the
guild gets the defaults, exactly as the old reader did.

During a rolling deploy, an old shard can still write a guild's Redis record
after a new shard has already moved it to Postgres. That edit is lost. The
window is the length of the rollout and only affects guilds edited during it.

### Optional sweep

Guilds that are never read again are never moved. To move them, or to see
which records are unreadable, run the sweep. It is safe while the bot is
running: it inserts rows only where none exist and deletes each Redis record
once its guild has a row.

```sh
go build -o /tmp/migrate-guild-settings ./cmd/migrate-guild-settings
/tmp/migrate-guild-settings --dry-run   # validate only; touches nothing
/tmp/migrate-guild-settings             # move everything that validates
```

The bot image also includes it, so a Docker Compose install can run it
without Go:

```sh
docker compose run --rm --no-deps --entrypoint ./migrate-guild-settings automuteus --dry-run
```

It reads the deployment's `REDIS_ADDR`, `REDIS_PASS`, `POSTGRES_ADDR`,
`POSTGRES_USER`, and `POSTGRES_PASS`, plus an optional `REDIS_USER`, and uses
Redis database 0 like the bot. It prints a JSON report and exits nonzero if
any record failed; failed records stay in Redis for inspection. The sweep only
touches `automuteus:settings:guild:*` keys.

9.2.x is the last release with this migration. 10.0 no longer reads settings
from Redis, so run the sweep until it reports nothing left before upgrading.

## Tests

`go test ./...` runs the unit tests. The live tests also run when both
disposable services are configured, as CI does:

```sh
TEST_REDIS_ADDR=localhost:6379 \
TEST_POSTGRES_URL='postgres://postgres:settings-test@localhost:5432/postgres?sslmode=disable' \
go test ./...
```

The live tests create and delete their own guild records and scan the legacy
settings namespace, so never point them at a real deployment.
