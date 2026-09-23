-- Guild settings, keyed by the same SHA-256 guild ID hash the legacy Redis
-- records used. Safe to apply repeatedly; the advisory lock serializes DDL
-- across bot processes that start at the same time.
--
-- voice_rules and delays are NULL when a guild uses the built-in defaults.
-- The defaults live in Go (pkg/game); the database never resolves them.
-- A guild with no row at all uses the defaults for everything.
--
-- Adding a setting: add a column here with ADD COLUMN IF NOT EXISTS, then add
-- it to settingsColumns and the scan/args helpers in guild_settings.go.
BEGIN;
SELECT pg_advisory_xact_lock(754810023);

CREATE TABLE IF NOT EXISTS guild_settings (
    guild_hash text PRIMARY KEY CHECK (guild_hash ~ '^[0-9a-f]{64}$'),
    admin_user_ids text[],
    permission_role_ids text[],
    language text NOT NULL,
    voice_rules jsonb,
    map_version text NOT NULL,
    delays jsonb,
    delete_game_summary_minutes bigint NOT NULL,
    unmute_dead_during_tasks boolean NOT NULL,
    auto_refresh boolean NOT NULL,
    match_summary_channel_id text NOT NULL,
    leaderboard_mention boolean NOT NULL,
    leaderboard_size bigint NOT NULL,
    leaderboard_min bigint NOT NULL,
    mute_spectator boolean NOT NULL,
    display_room_code text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1
);
-- version counts writes to the row (1 on creation); the API uses it for ETags and conditional writes.
ALTER TABLE guild_settings ADD COLUMN IF NOT EXISTS version bigint NOT NULL DEFAULT 1;
COMMIT;
