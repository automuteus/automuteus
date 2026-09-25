package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/top-gg/go-dbl"
)

// The columns each Postgres* type reads. Queries name them instead of using SELECT *, because scany refuses a
// column the struct has no field for, so a column added to a table by a newer release would otherwise break
// every read of that table here.
const (
	guildColumns = "guild_id, guild_name, premium, tx_time_unix, transferred_to, inherits_from"
	userColumns  = "user_id, opt, vote_time_unix"
	gameColumns  = "game_id, guild_id, connect_code, start_time, win_type, end_time, play_map, region"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	Ping(context.Context) error
	Prepare(context.Context, string, string) (*pgconn.StatementDescription, error)
}

type PsqlInterface struct {
	Pool *pgxpool.Pool

	// TODO does this require a lock? How should stuff be written/read from psql in an async way? Is this even a concern?
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

func (psqlInterface *PsqlInterface) Init(addr string) error {
	dbpool, err := pgxpool.Connect(context.Background(), addr)
	if err != nil {
		return err
	}
	psqlInterface.Pool = dbpool
	return nil
}

func insertGuild(conn PgxIface, guildID uint64, guildName string) error {
	_, err := conn.Exec(context.Background(), "INSERT INTO guilds VALUES ($1, $2, 0);", guildID, guildName)
	return err
}

func getGuild(conn PgxIface, guildID uint64) (*PostgresGuild, error) {
	var guilds []*PostgresGuild
	err := pgxscan.Select(context.Background(), conn, &guilds, "SELECT "+guildColumns+" FROM guilds WHERE guild_id = $1", guildID)
	if err != nil {
		return nil, err
	}

	if len(guilds) > 0 {
		return guilds[0], nil
	}
	return nil, errors.New("no guild found by that ID")
}

func insertUser(conn PgxIface, userID uint64) error {
	_, err := conn.Exec(context.Background(), "INSERT INTO users VALUES ($1, true, NULL)", userID)
	return err
}

func (psqlInterface *PsqlInterface) OptUserByString(userID string, opt bool) error {
	uid, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return err
	}
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	return optUser(conn.Conn(), uid, opt)
}

func optUser(conn PgxIface, uid uint64, opt bool) error {
	user, err := ensureUserExists(conn, uid)
	if err != nil {
		return err
	}
	if user.Opt == opt {
		return errors.New("user opt status is already set to the value specified")
	}
	_, err = conn.Exec(context.Background(), "UPDATE users SET opt = $1 WHERE user_id = $2;", opt, uid)
	if err != nil {
		return err
	}
	if !opt {
		_, err = conn.Exec(context.Background(), "UPDATE game_events SET user_id = NULL WHERE user_id = $1;", uid)
		if err != nil {
			return err
		}

		_, err = conn.Exec(context.Background(), "DELETE FROM users_games WHERE user_id = $1;", uid)
		if err != nil {
			return err
		}
	}

	return nil
}

func setUserVoteTime(conn PgxIface, userID string, timeUnix int64) error {
	uid, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return err
	}
	user, err := ensureUserExists(conn, uid)
	if err != nil {
		return err
	}
	if user.VoteTimeUnix != nil {
		return errors.New("user already has a vote time recorded in the DB")
	}
	_, err = conn.Exec(context.Background(), "UPDATE users SET vote_time_unix = $1 WHERE user_id = $2;", timeUnix, uid)
	return err
}

func (psqlInterface *PsqlInterface) GetUserByString(userID string) (*PostgresUser, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return getUserByString(conn.Conn(), userID)
}

func getUserByString(conn PgxIface, userID string) (*PostgresUser, error) {
	uid, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, err
	}
	return getUser(conn, uid)
}

func getUser(conn PgxIface, userID uint64) (*PostgresUser, error) {
	var users []*PostgresUser
	err := pgxscan.Select(context.Background(), conn, &users, "SELECT "+userColumns+" FROM users WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	if len(users) > 0 {
		return users[0], nil
	}
	return nil, fmt.Errorf("no user found with ID %d", userID)
}

func insertGame(conn PgxIface, game *PostgresGame) (uint64, error) {
	t, err := conn.Query(context.Background(), "INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time, play_map, region) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING game_id;", game.GuildID, game.ConnectCode, game.StartTime, game.WinType, game.EndTime, game.PlayMap, game.Region)
	if t != nil {
		if t.Next() {
			g := uint64(0)
			err := t.Scan(&g)

			if err != nil {
				log.Println(err)
				t.Close()
				return 0, err
			}
			t.Close()
			return g, nil
		}
	}
	return 0, err
}

func updateGame(conn PgxIface, gameID int64, winType int16, endTime int64) error {
	_, err := conn.Exec(context.Background(), "UPDATE games SET (win_type, end_time) = ($1, $2) WHERE game_id = $3;", winType, endTime, gameID)
	return err
}

func insertPlayer(conn PgxIface, player *PostgresUserGame) error {
	_, err := conn.Exec(context.Background(), "INSERT INTO users_games VALUES ($1, $2, $3, $4, $5, $6, $7);", player.UserID, player.GuildID, player.GameID, player.PlayerName, player.PlayerColor, player.PlayerRole, player.PlayerWon)
	return err
}

const (
	SecsInADay  = 86400
	SecsIn12Hrs = SecsInADay / 2
	TopGGID     = "753795015830011944"
)

func isUserPremium(conn PgxIface, dbl *dbl.Client, userID string) (bool, error) {
	// first check Postgres, because top.gg has ratelimits
	u, err := getUserByString(conn, userID)
	if err != nil {
		return false, err
	}
	if u.VoteTimeUnix != nil {
		// only premium if the first time they voted is within the last 12 hours
		diff := time.Now().Unix() - int64(*u.VoteTimeUnix)
		return diff < SecsIn12Hrs, nil
	}
	if dbl == nil {
		return false, nil
	}
	// only check if the user has never voted before
	voted, err := dbl.HasUserVoted(TopGGID, userID)
	if err != nil {
		return false, err
	}
	if voted {
		// do this in the background so the overall check is quick. We can overwrite because we know that tx_time=nil
		go func() {
			err := setUserVoteTime(conn, userID, time.Now().Unix())
			if err != nil {
				log.Println(err)
			}
		}()
		return true, nil
	}
	return false, nil
}

func (psqlInterface *PsqlInterface) GetGuildOrUserPremiumStatus(official bool, dbl *dbl.Client, guildID, userID string) (premium.Tier, int, error) {
	if !official {
		return premium.SelfHostTier, premium.NoExpiryCode, nil
	}
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return premium.FreeTier, 0, err
	}
	defer conn.Release()

	return guildOrUserPremium(conn.Conn(), dbl, guildID, userID)
}

func guildOrUserPremium(conn PgxIface, dbl *dbl.Client, guildID, userID string) (premium.Tier, int, error) {
	tier, daysRem := getGuildPremiumStatus(conn, guildID, 0)
	// only check the user premium if the guild doesn't have it
	if premium.IsExpired(tier, daysRem) && userID != "" {
		prem, err := isUserPremium(conn, dbl, userID)
		if err != nil {
			log.Println(err)
		}
		if prem {
			// no expiry because the expiry is handled per-user elsewhere
			return premium.TrialTier, premium.NoExpiryCode, nil
		}
	}
	return tier, daysRem, nil
}

func getGuildPremiumStatus(conn PgxIface, guildID string, depth int) (premium.Tier, int) {
	tier, days, err := checkedGuildPremiumStatus(context.Background(), conn, guildID, depth)
	if err != nil {
		log.Println(err)
		return premium.FreeTier, 0
	}
	return tier, days
}

func (psqlInterface *PsqlInterface) EnsureGuildExists(guildID uint64, guildName string) (*PostgresGuild, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	guild, err := getGuild(conn.Conn(), guildID)

	if guild == nil {
		err := insertGuild(conn.Conn(), guildID, guildName)
		if err != nil {
			return nil, err
		}
		return getGuild(conn.Conn(), guildID)
	}
	return guild, err
}

func (psqlInterface *PsqlInterface) EnsureUserExists(userID uint64) (*PostgresUser, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return ensureUserExists(conn.Conn(), userID)
}

func ensureUserExists(conn PgxIface, userID uint64) (*PostgresUser, error) {
	user, err := getUser(conn, userID)

	if user == nil {
		err := insertUser(conn, userID)
		if err != nil {
			log.Println(err)
		}
		return getUser(conn, userID)
	}
	return user, err
}

func (psqlInterface *PsqlInterface) AddInitialGame(game *PostgresGame) (uint64, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	return insertGame(conn.Conn(), game)
}

func (psqlInterface *PsqlInterface) AddEvent(event *PostgresGameEvent) error {
	if event.UserID == nil {
		_, err := psqlInterface.Pool.Exec(context.Background(), "INSERT INTO game_events VALUES (DEFAULT, NULL, $1, $2, $3, $4);", event.GameID, event.EventTime, event.EventType, event.Payload)
		return err
	}
	_, err := psqlInterface.Pool.Exec(context.Background(), "INSERT INTO game_events VALUES (DEFAULT, $1, $2, $3, $4, $5);", event.UserID, event.GameID, event.EventTime, event.EventType, event.Payload)
	return err
}

// make sure to call the relevant "ensure" methods before this one...
func (psqlInterface *PsqlInterface) UpdateGameAndPlayers(gameID int64, winType int16, endTime int64, players []*PostgresUserGame) error {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	err = updateGame(conn.Conn(), gameID, winType, endTime)
	if err != nil {
		return err
	}

	for _, player := range players {
		err := insertPlayer(conn.Conn(), player)
		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

// AbortGame marks a match as ended without a result. It is excluded from statistics.
func (psqlInterface *PsqlInterface) AbortGame(gameID int64, endTime int64) error {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()
	return updateGame(conn.Conn(), gameID, int16(game.Aborted), endTime)
}

func (psqlInterface *PsqlInterface) Close() {
	psqlInterface.Pool.Close()
}
