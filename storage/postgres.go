package storage

import (
	"context"
	"fmt"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

type PsqlInterface struct {
	pool *pgxpool.Pool

	//TODO does this require a lock? How should stuff be written/read from psql in an async way? Is this even a concern?
	//https://brandur.org/postgres-connections
}

type PremiumTier int16

const (
	FreeTier PremiumTier = iota
	BronzeTier
	SilverTier
	GoldTier
	PlatTier
	SelfHostTier
)

var PremiumTierStrings = []string{
	"Free",
	"Bronze",
	"Silver",
	"Gold",
	"Platinum",
	"SelfHost",
}

func ConstructPsqlConnectURL(addr, username, password string) string {
	return fmt.Sprintf("postgres://%s?user=%s&password=%s", addr, username, password)
}

type PsqlParameters struct {
	Addr     string
	Username string
	Password string
}

var psqlctx = context.Background()

func (psqlInterface *PsqlInterface) Init(addr string) error {
	dbpool, err := pgxpool.Connect(context.Background(), addr)
	if err != nil {
		return err
	}
	psqlInterface.pool = dbpool
	return nil
}

func (psqlInterface *PsqlInterface) LoadAndExecFromFile(filepath string) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	tag, err := psqlInterface.pool.Exec(context.Background(), string(bytes))
	if err != nil {
		return err
	}
	log.Println(tag.String())
	return nil
}

func (psqlInterface *PsqlInterface) insertGuild(guildID uint64, guildName string) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO guilds VALUES ($1, $2, 0);", guildID, guildName)
	return err
}

func (psqlInterface *PsqlInterface) GetGuild(guildID uint64) (*PostgresGuild, error) {
	guilds := []*PostgresGuild{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &guilds, "SELECT * FROM guilds WHERE guild_id=$1", guildID)
	if err != nil {
		return nil, err
	}

	if len(guilds) > 0 {
		return guilds[0], nil
	}
	return nil, nil
}

func (psqlInterface *PsqlInterface) insertUser(userID uint64) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO users VALUES ($1, true, DEFAULT);", userID)
	return err
}

func (psqlInterface *PsqlInterface) OptUserByString(userID string, opt bool) (bool, error) {
	uid, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return false, err
	}
	user, err := psqlInterface.EnsureUserExists(uid)
	if err != nil {
		return false, err
	}
	if user.Opt == opt {
		return false, nil
	}
	_, err = psqlInterface.pool.Exec(context.Background(), "UPDATE users SET (opt) = ($1) WHERE user_id = $2;", opt, uid)
	if err != nil {
		return false, err
	}
	if !opt {
		_, err = psqlInterface.pool.Exec(context.Background(), "UPDATE game_events SET (hashed_user_id) = (NULL) WHERE hashed_user_id = $1;", user.HashedUserID)
		if err != nil {
			log.Println(err)
		}
	}

	return true, nil
}

func (psqlInterface *PsqlInterface) GetUserByString(userID string) (*PostgresUser, error) {
	uid, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, err
	}
	return psqlInterface.GetUser(uid)
}

func (psqlInterface *PsqlInterface) GetUser(userID uint64) (*PostgresUser, error) {
	users := []*PostgresUser{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &users, "SELECT * FROM users WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	if len(users) > 0 {
		return users[0], nil
	}
	return nil, nil
}

func (psqlInterface *PsqlInterface) insertGame(game *PostgresGame) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO games VALUES ($1, $2, $3, $4, $5, $6);", game.GameID, game.GuildID, game.ConnectCode, game.StartTime, game.WinType, game.EndTime)
	return err
}

func (psqlInterface *PsqlInterface) updateGame(gameID int64, winType int16, endTime int64) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "UPDATE games SET (win_type, end_time) = ($1, $2) WHERE game_id = $3;", winType, endTime, gameID)
	return err
}

func (psqlInterface *PsqlInterface) insertPlayer(player *PostgresUserGame) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO users_games VALUES ($1, $2, $3, $4, $5, $6, $7);", player.UserID, player.GuildID, player.GameID, player.PlayerName, player.PlayerColor, player.PlayerRole, player.PlayerWon)
	return err
}

func (psqlInterface *PsqlInterface) GetGuildPremiumStatus(guildID string) PremiumTier {
	//self-hosting; only return the true guild status if this variable is set
	if os.Getenv("OFFICIAL") == "" {
		return SelfHostTier
	}

	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		log.Println(err)
		return FreeTier
	}

	guild, err := psqlInterface.GetGuild(gid)
	if err != nil {
		return FreeTier
	}
	return PremiumTier(guild.Premium)
}

func (psqlInterface *PsqlInterface) EnsureGuildExists(guildID uint64, guildName string) (*PostgresGuild, error) {
	guild, err := psqlInterface.GetGuild(guildID)

	if guild == nil {
		err := psqlInterface.insertGuild(guildID, guildName)
		if err != nil {
			return nil, err
		}
		return psqlInterface.GetGuild(guildID)
	}
	return guild, err
}

func (psqlInterface *PsqlInterface) EnsureUserExists(userID uint64) (*PostgresUser, error) {
	user, err := psqlInterface.GetUser(userID)

	if user == nil {
		err := psqlInterface.insertUser(userID)
		if err != nil {
			log.Println(err)
		}
		return psqlInterface.GetUser(userID)
	}
	return user, err
}

func (psqlInterface *PsqlInterface) AddInitialGame(game *PostgresGame) error {
	err := psqlInterface.insertGame(game)
	if err != nil {
		return err
	}

	return nil
}

func (psqlInterface *PsqlInterface) AddEvent(event *PostgresGameEvent) error {
	if event.HashedUserID < 0 {
		_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO game_events VALUES (DEFAULT, NULL, $1, $2, $3, $4);", event.GameID, event.EventTime, event.EventType, event.Payload)
		return err
	}
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO game_events VALUES (DEFAULT, $1, $2, $3, $4, $5);", event.HashedUserID, event.GameID, event.EventTime, event.EventType, event.Payload)
	return err
}

//make sure to call the relevant "ensure" methods before this one...
func (psqlInterface *PsqlInterface) UpdateGameAndPlayers(gameID int64, winType int16, endTime int64, players []*PostgresUserGame) error {

	err := psqlInterface.updateGame(gameID, winType, endTime)
	if err != nil {
		return err
	}

	for _, player := range players {
		err := psqlInterface.insertPlayer(player)
		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

func (psqlInterface *PsqlInterface) Close() {
	psqlInterface.pool.Close()
}
