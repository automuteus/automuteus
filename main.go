package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/bot/tokenprovider"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/locale"
	"github.com/automuteus/automuteus/v8/pkg/logging"
	storage2 "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"

	"github.com/automuteus/automuteus/v8/storage"

	"github.com/automuteus/automuteus/v8/bot"
)

var (
	version = "v9.0.0"
	commit  = "none"
	date    = "unknown"
)

const (
	DefaultURL                   = "http://localhost:8123"
	DefaultMaxRequests5Sec int64 = 5 // Discord allows ~10 member modifications per 10s per guild
)

type registeredCommand struct {
	GuildID            string
	ApplicationCommand *discordgo.ApplicationCommand
}

func main() {
	// seed the rand generator (used for making connection codes)
	rand.Seed(time.Now().Unix())
	err := discordMainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		return
	}
}

func discordMainWrapper() error {
	var isOfficial = os.Getenv("AUTOMUTEUS_OFFICIAL") != ""

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./"
	}

	var logOut io.Writer = os.Stdout
	if os.Getenv("DISABLE_LOG_FILE") == "" {
		file, err := os.Create(path.Join(logPath, "logs.txt"))
		if err != nil {
			return err
		}
		logOut = io.MultiWriter(os.Stdout, file)
	}
	logging.Setup(logOut)

	emojiGuildID := os.Getenv("EMOJI_GUILD_ID")

	log.Println(version + "-" + commit)

	numShardsStr := os.Getenv("NUM_SHARDS")
	numShards, err := strconv.Atoi(numShardsStr)
	if err != nil {
		log.Println("No NUM_SHARDS specified; defaulting to 1")
		numShards = 1
	}

	shardIDStr := os.Getenv("SHARD_ID")
	if shardIDStr != "" {
		return errors.New("SHARD_ID is no longer supported! Please use SHARDS instead")
	}

	var shards shards
	shardsStr := os.Getenv("SHARDS")
	if shardsStr == "" {
		log.Println("No SHARDS specified, defaulting to 0")
		shards = defaultShard()
	} else {
		shards, err = parseShards(shardsStr, numShards)
		if err != nil {
			return err
		}
	}

	url := os.Getenv("HOST")
	if url == "" {
		log.Printf("[Info] No valid HOST provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
	}

	var redisClient bot.RedisInterface

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASS")
	if redisAddr != "" {
		err := redisClient.Init(storage.RedisParameters{
			Addr:     redisAddr,
			Username: "",
			Password: redisPassword,
		})
		if err != nil {
			log.Println(err)
		}
	} else {
		return errors.New("no REDIS_ADDR specified; exiting")
	}

	if os.Getenv("LOCALE_PATH") != "" {
		log.Println("LOCALE_PATH is deprecated and ignored: translations are embedded in the binary")
	}
	locale.InitLang(os.Getenv("BOT_LANG"))

	psql := storage2.PsqlInterface{}
	pAddr := os.Getenv("POSTGRES_ADDR")
	if pAddr == "" {
		return errors.New("no POSTGRES_ADDR specified; exiting")
	}

	pUser := os.Getenv("POSTGRES_USER")
	if pUser == "" {
		return errors.New("no POSTGRES_USER specified; exiting")
	}

	pPass := os.Getenv("POSTGRES_PASS")
	if pPass == "" {
		return errors.New("no POSTGRES_PASS specified; exiting")
	}

	err = psql.Init(storage2.ConstructPsqlConnectURL(pAddr, pUser, pPass))
	if err != nil {
		return err
	}

	defer psql.Pool.Close()
	// The API and bot can start concurrently against a fresh self-hosted DB.
	schemaCtx, cancelSchema := context.WithTimeout(context.Background(), time.Minute)
	err = storage.ApplySchemas(schemaCtx, psql.Pool, isOfficial)
	cancelSchema()
	if err != nil {
		return err
	}
	// Settings that are still in Redis from older versions are moved to
	// Postgres the first time each guild is read.
	legacySettings := redis.NewClient(&redis.Options{Addr: redisAddr, Password: redisPassword})
	defer legacySettings.Close()
	storageInterface := storage.NewPostgresStorage(psql.Pool, legacySettings)

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	go server.StartHealthCheckServer("8080")

	topGGToken := os.Getenv("TOP_GG_TOKEN")

	taskTimeoutms := capture.DefaultCaptureBotTimeout

	taskTimeoutmsStr := os.Getenv("ACK_TIMEOUT_MS")
	num, err := strconv.ParseInt(taskTimeoutmsStr, 10, 64)
	if err == nil {
		log.Printf("Read from env; using ACK_TIMEOUT_MS=%d\n", num)
		taskTimeoutms = time.Millisecond * time.Duration(num)
	}

	maxReq5Sec := os.Getenv("MAX_REQ_5_SEC")
	maxReq := DefaultMaxRequests5Sec
	num, err = strconv.ParseInt(maxReq5Sec, 10, 64)
	if err == nil {
		maxReq = num
	}

	tokenProvider := tokenprovider.NewTokenProvider(nil, nil, taskTimeoutms, maxReq)
	var extraTokens []string
	extraTokenStr := strings.ReplaceAll(os.Getenv("WORKER_BOT_TOKENS"), " ", "")
	if extraTokenStr != "" {
		extraTokens = strings.Split(extraTokenStr, ",")
	}

	bots := make([]*bot.Bot, len(shards))
	for i, shard := range shards {
		bots[i] = bot.MakeAndStartBot(version, commit, discordToken, topGGToken, url, emojiGuildID, numShards, int(shard), &redisClient, storageInterface, &psql, logPath)
		if bots[i] == nil {
			log.Fatalf("bot %d failed to initialize; did you provide a valid Discord Bot Token?", shard)
		}
	}

	// initialize the token provider using the first shard's redis client and primary session
	bots[0].InitTokenProvider(tokenProvider)
	for i := 0; i < len(shards); i++ {
		bots[i].SetTokenProvider(tokenProvider)
	}
	tokenProvider.PopulateAndStartSessions(extraTokens)
	// indicate to Kubernetes that we're ready to start receiving traffic
	server.GlobalReady = true

	go func() {
		if err := server.PrometheusMetricsServer("2112"); err != nil {
			log.Printf("Metrics server stopped: %v", err)
		}
	}()

	// empty string entry = global
	slashCommandGuildIds := []string{""}
	slashCommandGuildIdStr := strings.ReplaceAll(os.Getenv("SLASH_COMMAND_GUILD_IDS"), " ", "")
	if slashCommandGuildIdStr != "" {
		slashCommandGuildIds = strings.Split(slashCommandGuildIdStr, ",")
	}

	// only register commands if we're not the official bot, OR we're the primary/main shard
	var registeredCommands []registeredCommand
	if !isOfficial || shards.isPrimaryShard() {
		for _, guild := range slashCommandGuildIds {
			for _, v := range command.All {
				if guild == "" {
					log.Printf("Registering command %s GLOBALLY\n", v.Name)
				} else {
					log.Printf("Registering command %s in guild %s\n", v.Name, guild)
				}

				id, err := bots[0].PrimarySession.ApplicationCommandCreate(bots[0].PrimarySession.State.User.ID, guild, v)
				if err != nil {
					log.Panicf("Cannot create command: %v", err)
				} else {
					registeredCommands = append(registeredCommands, registeredCommand{
						GuildID:            guild,
						ApplicationCommand: id,
					})
				}
			}
		}
		log.Println("Finishing registering all commands!")
	}

	<-sc
	log.Printf("Received Sigterm or Kill signal. Bot will terminate in 1 second")
	time.Sleep(time.Second)

	// only delete the slash commands if we're not the official bot, AND we're the primary/"master" shard
	if !isOfficial && shards.isPrimaryShard() {
		log.Println("Deleting slash commands")
		for _, v := range registeredCommands {
			if v.GuildID == "" {
				log.Printf("Deleting command %s GLOBALLY\n", v.ApplicationCommand.Name)
			} else {
				log.Printf("Deleting command %s on guild %s\n", v.ApplicationCommand.Name, v.GuildID)
			}
			err = bots[0].PrimarySession.ApplicationCommandDelete(v.ApplicationCommand.ApplicationID, v.GuildID, v.ApplicationCommand.ID)
			if err != nil {
				log.Println(err)
			}
		}
		log.Println("Finished deleting all commands")
	}

	for _, v := range bots {
		v.Close()
	}
	tokenProvider.Close()
	return nil
}

type shards []uint8

func defaultShard() shards {
	return []uint8{0}
}

// isPrimaryShard ensures that the FIRST shard running is the 0th/primary shard.
// This prevents performing additional work when shard instances may overlap
// (for example, an instance running 0,1, and another running 1,0)
func (sr shards) isPrimaryShard() bool {
	return len(sr) > 0 && sr[0] == 0
}

func parseShards(str string, maxShards int) (shards, error) {
	var shards shards

	tokens := strings.Split(strings.ReplaceAll(str, " ", ""), ",")
	for _, token := range tokens {
		v, err := strconv.ParseUint(token, 10, 64)
		if err != nil {
			return shards, err
		}
		if v >= uint64(maxShards) {
			return shards, fmt.Errorf("shard: %d is greater or equal to the total max shards: %d", v, maxShards)
		}
		shards = append(shards, uint8(v))
	}
	return shards, nil
}
