-- Run by hand, once, on the official database, as the role that owns the stats tables. It is not applied at startup:
-- self-hosted installs get the same result from postgres.sql, which drops these inside its startup transaction.
--
-- Here every statement uses CONCURRENTLY so nothing blocks the running bot, which means this file must NOT be run
-- inside a transaction (psql without --single-transaction; one statement at a time). Any statement can be re-run:
-- they are all "if exists" / "if not exists".
--
-- What goes, and why (sizes and scan counts from production on 2026-11-24, 75 days after the last stats reset):
--   guilds_id_index            6 MB    copy of guilds_pkey
--   games_game_id_index        80 MB   copy of games_pkey
--   users_user_id_index        31 MB   copy of users_pkey
--   users_games_user_id_index  1089 MB leading column of users_games_pkey (user_id, game_id)
--   users_games_won_index      676 MB  0 scans; a boolean
--   users_games_role_index     677 MB  161 scans; two values, and every query filters by guild first
--   games_win_type_index       109 MB  0 scans
--   games_connect_code_index   161 MB  0 scans
--   game_events_user_id_index  2230 MB 70% of its entries are null user IDs; replaced by a partial index
--
-- The planner picked the copies over the primary keys only because they are the same size or smaller; with the
-- copies gone it uses the primary keys.

SET lock_timeout = '10s';

-- 1. Copies and unused indexes. Each drop is immediate once it gets its lock.
DROP INDEX CONCURRENTLY IF EXISTS guilds_id_index;
DROP INDEX CONCURRENTLY IF EXISTS games_game_id_index;
DROP INDEX CONCURRENTLY IF EXISTS users_user_id_index;
DROP INDEX CONCURRENTLY IF EXISTS users_games_user_id_index;
DROP INDEX CONCURRENTLY IF EXISTS users_games_won_index;
DROP INDEX CONCURRENTLY IF EXISTS users_games_role_index;
DROP INDEX CONCURRENTLY IF EXISTS games_win_type_index;
DROP INDEX CONCURRENTLY IF EXISTS games_connect_code_index;

-- 2. Build the partial index next to the old one. This reads all of game_events twice; expect several minutes.
CREATE INDEX CONCURRENTLY IF NOT EXISTS game_events_user_id_notnull_index
    ON game_events (user_id) WHERE user_id IS NOT NULL;

-- A build that was interrupted leaves an invalid index behind. This must return no rows; if it returns one, drop
-- that index (concurrently) and build it again.
SELECT indexrelid::regclass AS invalid_index FROM pg_index WHERE NOT indisvalid;

-- 3. Check the opt-out lookup uses the new index before the old one goes. The plan must name
-- game_events_user_id_notnull_index, not a sequential scan. (EXPLAIN without ANALYZE changes nothing, and any
-- user ID gives the same plan.)
EXPLAIN UPDATE game_events SET user_id = NULL WHERE user_id = 1;

-- 4. Only once step 3 shows the new index:
DROP INDEX CONCURRENTLY IF EXISTS game_events_user_id_index;

-- Afterwards: about 4.3 GB smaller.
SELECT pg_size_pretty(pg_database_size(current_database())) AS database_size;
