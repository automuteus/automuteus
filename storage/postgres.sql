-- Applied automatically only on self-hosted installs (storage.ApplySchemas skips it for official deployments).
-- Official production applies schema changes by hand: when you change this file, put the exact SQL in the PR's
-- deploy notes, to run before the bot deploys, or the bot's writes fail against the old schema.

create table if not exists guilds
(
    guild_id numeric PRIMARY KEY,
    guild_name VARCHAR(100) NOT NULL,
    premium smallint NOT NULL,
    tx_time_unix integer,
    transferred_to numeric references guilds(guild_id),
    inherits_from numeric references guilds(guild_id)
);

create table if not exists games
(
    game_id      bigserial PRIMARY KEY,
    guild_id     numeric references guilds ON DELETE CASCADE, --if the guild is deleted, delete their games, too
    connect_code CHAR(8) NOT NULL,
    start_time   integer NOT NULL,                            --2038 problem, but I do not care
    win_type     smallint,                                    --imposter win, crewmate win, etc
    end_time     integer,                                     --2038 problem, but I do not care
    play_map     smallint,                                    --game.PlayMap at match start; NULL if unknown
    region       smallint,                                    --game.Region at match start; NULL if unknown
    capture_kind    text,                                     --capture.KindApp/KindMod at match start; NULL if unreported
    capture_version text                                      --that capture client's version; NULL if unreported
);

-- add columns to games created before they existed. Checking the catalog first skips the ALTER, and its brief
-- exclusive lock on games, on every startup after the first.
do $$
begin
    if not exists (select 1 from information_schema.columns
                   where table_schema = current_schema() and table_name = 'games' and column_name = 'play_map') then
        alter table games add column play_map smallint;
    end if;
    if not exists (select 1 from information_schema.columns
                   where table_schema = current_schema() and table_name = 'games' and column_name = 'region') then
        alter table games add column region smallint;
    end if;
    if not exists (select 1 from information_schema.columns
                   where table_schema = current_schema() and table_name = 'games' and column_name = 'capture_kind') then
        alter table games add column capture_kind text, add column capture_version text;
    end if;
end
$$;

-- links userIDs to their hashed variants. Allows for deletion of users without deleting underlying game_event data
create table if not exists users
(
    user_id numeric PRIMARY KEY,
    opt     boolean, --opt-out to data collection
    vote_time_unix integer --unused since the top.gg voting trial was removed; insertUser still fills it positionally
);

-- add vote_time_unix column if not exists due to upgrade from 7.1.2 or earlier
alter table users add column if not exists vote_time_unix integer;

create table if not exists game_events
(
    event_id   bigserial,
    user_id    numeric,                                              --actually references users, but can be null, so implied reference, not literal
    game_id    bigint   NOT NULL references games ON DELETE CASCADE, --delete all events from a game that's deleted
    event_time integer  NOT NULL,                                    --2038 problem, but I do not care
    event_type smallint NOT NULL,
    payload    jsonb
);

create table if not exists users_games
(
    user_id numeric REFERENCES users ON DELETE CASCADE, --if a user gets deleted, delete their linked games
    guild_id numeric REFERENCES guilds ON DELETE CASCADE, --if a guild is deleted, delete all linked games
    game_id bigint REFERENCES games ON DELETE CASCADE, --if a game is deleted, delete all linked users_games
    player_name VARCHAR(10) NOT NULL,
    player_color smallint NOT NULL,
    player_role smallint NOT NULL,
    player_won bool NOT NULL,
    PRIMARY KEY (user_id, game_id)
);

-- Lookups by a table's own ID use its primary key; users_games (user_id, game_id) also serves lookups by user.
create index if not exists guilds_premium_index ON guilds (premium); --query guilds by prem status

create index if not exists games_guild_id_index ON games (guild_id); --query games by guild ID

create index if not exists users_games_game_id_index ON users_games (game_id); --query games by game ID
create index if not exists users_games_guild_id_index ON users_games (guild_id); --query games by guild ID

create index if not exists game_events_game_id_index on game_events (game_id); --query for game events by the game ID
-- Most events carry no user (the player was never linked), and nothing looks those up by user, so only linked
-- events are indexed. This is what finds a user's events when they opt out.
create index if not exists game_events_user_id_notnull_index on game_events (user_id) where user_id is not null;
-- Death events in match order, for the first-death stats (guildFirstTargetQuery in pkg/storage/guild_stats.go).
-- '2' is game.DIED; that query writes the action as a literal so the planner can match this predicate.
create index if not exists game_events_deaths_index on game_events (game_id, event_time, event_id) include (user_id)
    where (payload ->> 'Action') = '2';

-- Indexes that earlier versions created and no query used: copies of a primary key or of its leading column, and
-- single-column indexes on values every query reaches through the guild or game instead. Dropping one that is
-- already gone takes no lock, so this costs nothing after the first startup.
drop index if exists guilds_id_index;
drop index if exists games_game_id_index;
drop index if exists games_win_type_index;
drop index if exists games_connect_code_index;
drop index if exists users_user_id_index;
drop index if exists users_games_user_id_index;
drop index if exists users_games_role_index;
drop index if exists users_games_won_index;
drop index if exists game_events_user_id_index; --replaced by game_events_user_id_notnull_index