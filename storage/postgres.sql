create table if not exists guilds(
                                     guild_id     numeric PRIMARY KEY,
                                     guild_name   VARCHAR(100) NOT NULL,
                                     premium      smallint     NOT NULL,
                                     tx_time_unix integer
);

create table if not exists games
(
    game_id      bigserial PRIMARY KEY,
    guild_id     numeric references guilds ON DELETE CASCADE, --if the guild is deleted, delete their games, too
    connect_code CHAR(8) NOT NULL,
    start_time   integer NOT NULL,                            --2038 problem, but I do not care
    win_type     smallint,                                    --imposter win, crewmate win, etc
    end_time     integer                                      --2038 problem, but I do not care
);

-- links userIDs to their hashed variants. Allows for deletion of users without deleting underlying game_event data
create table if not exists users
(
    user_id numeric PRIMARY KEY,
    opt     boolean --opt-out to data collection
);

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
    user_id      numeric REFERENCES users ON DELETE CASCADE,  --if a user gets deleted, delete their linked games
    guild_id     numeric REFERENCES guilds ON DELETE CASCADE, --if a guild is deleted, delete all linked games
    game_id      bigint REFERENCES games ON DELETE CASCADE,   --if a game is deleted, delete all linked users_games
    player_name  VARCHAR(10) NOT NULL,
    player_color smallint    NOT NULL,
    player_role  smallint    NOT NULL,
    player_won   bool        NOT NULL,
    PRIMARY KEY (user_id, game_id)
);

create index if not exists guilds_id_index ON guilds (guild_id); --query guilds by ID
create index if not exists guilds_premium_index ON guilds (premium); --query guilds by prem status

create index if not exists games_game_id_index ON games (game_id); --query games by ID
create index if not exists games_guild_id_index ON games (guild_id); --query games by guild ID
create index if not exists games_win_type_index on games (win_type); --query games by win type
create index if not exists games_connect_code_index on games (connect_code); --query games by connect code

create index if not exists users_user_id_index ON users (user_id); --query for user info by their ID

create index if not exists users_games_user_id_index ON users_games (user_id); --query games by user ID
create index if not exists users_games_game_id_index ON users_games (game_id); --query games by game ID
create index if not exists users_games_guild_id_index ON users_games (guild_id); --query games by guild ID
create index if not exists users_games_role_index ON users_games (player_role); --query games by win status
create index if not exists users_games_won_index ON users_games (player_won); --query games by win status

create index if not exists game_events_game_id_index on game_events (game_id); --query for game events by the game ID
create index if not exists game_events_user_id_index on game_events (user_id); --query for game events by the user ID