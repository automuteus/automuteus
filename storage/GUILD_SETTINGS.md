# Guild settings in Postgres

Guild settings live in the `guild_settings` table. Redis holds game state,
queues, and locks, but not settings.

`guild_settings.sql` is embedded in the binary and applied on every startup.
It is idempotent and takes an advisory lock, so several shards can start at
once. The startup database role needs permission to create the table.

## Representation

- Rows are keyed by the SHA-256 guild ID hash, the same key the old Redis
  records used.
- Scalar settings have typed columns. `voice_rules` and `delays` are JSONB and
  are NULL when the guild uses the built-in defaults. The defaults are defined
  in Go (`pkg/game`); the database never resolves them. A guild with no row at
  all uses the defaults, so resetting settings deletes the row.
- Reads never substitute defaults for a failure. Storage or decoding errors are
  returned, and handlers tell the user to retry.
- Writes replace the whole document, so concurrent edits keep the old
  last-writer-wins behavior.

## Upgrading from a Redis-backed version

8.x and earlier stored settings in Redis under
`automuteus:settings:guild:<hash>`. 9.x moves each record into Postgres the
first time a guild is read, and ships `cmd/migrate-guild-settings` to move the
rest. 10.0 removed both, so it never reads Redis for settings. Upgrade to
9.2.x and run its sweep until it reports nothing left before upgrading
further; any record still in Redis afterwards is ignored and its guild uses
the defaults.

## Tests

`go test ./...` runs the unit tests. The live tests also run when a
disposable database is configured, as CI does:

```sh
TEST_POSTGRES_URL='postgres://postgres:settings-test@localhost:5432/postgres?sslmode=disable' \
go test ./...
```

The live tests create and delete their own guild records, so never point them
at a real deployment.
