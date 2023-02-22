package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/top-gg/go-dbl"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"
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
	tag, err := psqlInterface.Pool.Exec(context.Background(), string(bytes))
	if err != nil {
		return err
	}
	log.Println(tag.String())
	return nil
}

func insertGuild(conn PgxIface, guildID uint64, guildName string) error {
	_, err := conn.Exec(context.Background(), "INSERT INTO guilds VALUES ($1, $2, 0);", guildID, guildName)
	return err
}

func (psqlInterface *PsqlInterface) GetGuildForDownload(guildID uint64) (*PostgresGuild, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	guild, err := getGuild(conn.Conn(), guildID)
	if err != nil {
		return nil, err
	}
	guild.Premium = int16(premium.SelfHostTier)
	guild.TxTimeUnix = nil
	guild.InheritsFrom = nil
	guild.TransferredTo = nil
	return guild, nil
}

func getGuild(conn PgxIface, guildID uint64) (*PostgresGuild, error) {
	var guilds []*PostgresGuild
	err := pgxscan.Select(context.Background(), conn, &guilds, "SELECT * FROM guilds WHERE guild_id = $1", guildID)
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
	err := pgxscan.Select(context.Background(), conn, &users, "SELECT * FROM users WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	if len(users) > 0 {
		return users[0], nil
	}
	return nil, fmt.Errorf("no user found with ID %d", userID)
}

func (psqlInterface *PsqlInterface) GetGame(guildID, connectCode, matchID string) (*PostgresGame, error) {
	var games []*PostgresGame
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &games, "SELECT * FROM games WHERE guild_id = $1 AND game_id = $2 AND connect_code = $3;", guildID, matchID, connectCode)
	if err != nil {
		return nil, err
	}
	if len(games) > 0 {
		return games[0], nil
	}
	return nil, nil
}

func (psqlInterface *PsqlInterface) GetGameEvents(matchID string) ([]*PostgresGameEvent, error) {
	var events []*PostgresGameEvent
	err := pgxscan.Select(context.Background(), psqlInterface.Pool, &events, "SELECT * FROM game_events WHERE game_id = $1 ORDER BY event_id ASC;", matchID)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func insertGame(conn PgxIface, game *PostgresGame) (uint64, error) {
	t, err := conn.Query(context.Background(), "INSERT INTO games VALUES (DEFAULT, $1, $2, $3, $4, $5) RETURNING game_id;", game.GuildID, game.ConnectCode, game.StartTime, game.WinType, game.EndTime)
	if t != nil {
		for t.Next() {
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
	// if we somehow recurse too deep...
	if depth > 3 {
		return premium.FreeTier, 0
	}

	gid, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		log.Println(err)
		return premium.FreeTier, 0
	}

	guild, err := getGuild(conn, gid)
	if err != nil {
		log.Println(err)
		return premium.FreeTier, 0
	}

	// if this is a recursive call, then we ignore the transfer (this is how inheriting works)
	if depth == 0 {
		// transferred servers are always treated as free tier, even if their tier/expiry is marked otherwise (the server
		// that premium was transferred to still uses these values, as "inherited")
		if guild.TransferredTo != nil {
			return premium.FreeTier, 0
		}
	}

	daysRem := premium.NoExpiryCode

	if guild.TxTimeUnix != nil {
		diff := time.Now().Unix() - int64(*guild.TxTimeUnix)
		// 31 - days elapsed
		daysRem = int(premium.SubDays - (diff / SecsInADay))
		// if the premium for this server is still active, return it (disregarding inheritance)
		if daysRem > 0 {
			return premium.Tier(guild.Premium), daysRem
		}
	}

	// follow the link to the inherited server
	// other tooling that facilitates transfers/gold sub-servers will need to be careful to avoid cyclic inheritance...
	if guild.InheritsFrom != nil {
		return getGuildPremiumStatus(conn, fmt.Sprintf("%d", *guild.InheritsFrom), depth+1)
	}

	return premium.Tier(guild.Premium), daysRem
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

func (psqlInterface *PsqlInterface) GetGamesForGuild(guildID uint64) ([]*PostgresGame, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return getGamesForGuild(conn.Conn(), guildID)
}

func getGamesForGuild(conn PgxIface, guildID uint64) ([]*PostgresGame, error) {
	var games []*PostgresGame
	err := pgxscan.Select(context.Background(), conn, &games, "SELECT * FROM games WHERE guild_id = $1;", guildID)
	if err != nil {
		return nil, err
	}
	return games, nil
}

func (psqlInterface *PsqlInterface) GetGamesEventsForGuild(guildID uint64) ([]*PostgresGameEvent, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return getGameEventsForGuild(conn.Conn(), guildID)
}

func getGameEventsForGuild(conn PgxIface, guildID uint64) ([]*PostgresGameEvent, error) {
	var r []*PostgresGameEvent
	err := pgxscan.Select(context.Background(), conn, &r, "SELECT event_id, user_id, game_events.game_id, event_time, event_type, payload "+
		"FROM game_events "+
		"INNER JOIN games gg ON gg.game_id = game_events.game_id "+
		"WHERE gg.guild_id = $1", guildID)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (psqlInterface *PsqlInterface) GetUsersForGuild(guildID uint64) ([]*PostgresUser, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return getUsersForGuild(conn.Conn(), guildID)
}

func getUsersForGuild(conn PgxIface, guildID uint64) ([]*PostgresUser, error) {
	var r []*PostgresUser
	err := pgxscan.Select(context.Background(), conn, &r, "SELECT DISTINCT users.user_id,opt,vote_time_unix "+
		"FROM users "+
		"INNER JOIN game_events ge ON users.user_id = ge.user_id "+
		"INNER JOIN games gg ON gg.game_id = ge.game_id "+
		// only return users who are opted in to data collection
		"WHERE gg.guild_id = $1 AND users.opt = true", guildID)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (psqlInterface *PsqlInterface) GetUsersGamesForGuild(guildID uint64) ([]*PostgresUserGame, error) {
	conn, err := psqlInterface.Pool.Acquire(context.Background())
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return getUsersGamesForGuild(conn.Conn(), guildID)
}

func getUsersGamesForGuild(conn PgxIface, guildID uint64) ([]*PostgresUserGame, error) {
	var r []*PostgresUserGame
	err := pgxscan.Select(context.Background(), conn, &r, "SELECT DISTINCT users_games.user_id,guild_id,game_id,player_name,player_color,player_role,player_won "+
		"FROM users_games "+
		"INNER JOIN users u ON u.user_id = users_games.user_id "+
		"WHERE guild_id = $1 AND u.opt = true", guildID)
	if err != nil {
		return nil, err
	}
	return r, nil
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

func (psqlInterface *PsqlInterface) Close() {
	psqlInterface.Pool.Close()
}
