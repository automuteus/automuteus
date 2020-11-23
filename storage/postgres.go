package storage

import (
	"context"
	"fmt"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
	"io/ioutil"
	"log"
	"os"
)

type PsqlInterface struct {
	pool *pgxpool.Pool

	//TODO does this require a lock? How should stuff be written/read from psql in an async way? Is this even a concern?
	//https://brandur.org/postgres-connections
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

func (psqlInterface *PsqlInterface) insertGuild(guildID, guildName string) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO guilds VALUES ($1, $2, 'Free');", guildID, guildName)
	return err
}

func (psqlInterface *PsqlInterface) getGuild(guildID string) (*PostgresGuild, error) {
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

func (psqlInterface *PsqlInterface) insertUser(userID, hashedUserID string) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO users VALUES ($1, $2);", userID, hashedUserID)
	return err
}

func (psqlInterface *PsqlInterface) updateUser(userID, hashedUserID string) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "UPDATE users SET user_id = $1 WHERE hashed_user_id = $2", userID, hashedUserID)
	return err
}

func (psqlInterface *PsqlInterface) GetUser(userID string) (*PostgresUser, error) {
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

func (psqlInterface *PsqlInterface) getGuildUser(guildID, hashedID string) (*PostgresGuildUser, error) {
	users := []*PostgresGuildUser{}
	err := pgxscan.Select(context.Background(), psqlInterface.pool, &users, "SELECT * FROM guilds_users WHERE guild_id=$1 AND hashed_user_id=$2", guildID, hashedID)
	if err != nil {
		return nil, err
	}

	if len(users) > 0 {
		return users[0], nil
	}
	return nil, nil
}

func (psqlInterface *PsqlInterface) insertGuildUser(guildID, hashedID string) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO guilds_users VALUES ($1, $2);", guildID, hashedID)
	return err
}

func (psqlInterface *PsqlInterface) insertGuildGame(guildID string, gameID int64) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO guilds_games VALUES ($1, $2);", guildID, gameID)
	return err
}

func (psqlInterface *PsqlInterface) insertGame(game *PostgresGame) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO games VALUES ($1, $2, $3, $4, $5);", game.GameID, game.ConnectCode, game.StartTime, game.WinType, game.EndTime)
	return err
}

func (psqlInterface *PsqlInterface) updateGame(gameID int64, winType int16, endTime int64) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "UPDATE games SET (win_type, end_time) = ($1, $2) WHERE game_id = $3;", winType, endTime, gameID)
	return err
}

func (psqlInterface *PsqlInterface) insertPlayer(player *PostgresUserGame) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO users_games VALUES ($1, $2, $3, $4, $5);", player.HashedUserID, player.GameID, player.PlayerName, player.PlayerColor, player.PlayerRole)
	return err
}

func (psqlInterface *PsqlInterface) GetGuildPremiumStatus(guildID string) string {
	guild, err := psqlInterface.getGuild(guildID)
	if err != nil {
		return "Free"
	}
	return guild.Premium
}

func (psqlInterface *PsqlInterface) EnsureGuildExists(guildID, guildName string) error {
	guild, _ := psqlInterface.getGuild(guildID)

	if guild == nil {
		err := psqlInterface.insertGuild(guildID, guildName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (psqlInterface *PsqlInterface) EnsureUserExists(userID, hashedID string) error {
	user, _ := psqlInterface.GetUser(userID)

	if user == nil {
		err := psqlInterface.insertUser(userID, hashedID)
		if err != nil {
			err = psqlInterface.updateUser(userID, hashedID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (psqlInterface *PsqlInterface) RemoveUserMapping(userID string) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "UPDATE users SET user_id=NULL WHERE user_id=$1", userID)
	return err
}

func (psqlInterface *PsqlInterface) EnsureGuildUserExists(guildID, hashedID string) error {
	guild, _ := psqlInterface.getGuildUser(guildID, hashedID)

	if guild == nil {
		err := psqlInterface.insertGuildUser(guildID, hashedID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (psqlInterface *PsqlInterface) AddInitialGame(guildID string, game *PostgresGame) error {
	err := psqlInterface.insertGame(game)
	if err != nil {
		return err
	}

	err = psqlInterface.insertGuildGame(guildID, game.GameID)
	if err != nil {
		return err
	}
	return nil
}

func (psqlInterface *PsqlInterface) AddEvent(event *PostgresGameEvent) error {
	_, err := psqlInterface.pool.Exec(context.Background(), "INSERT INTO game_events VALUES (DEFAULT, $1, $2, $3, $4);", event.GameID, event.EventTime, event.EventType, event.Payload)
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
