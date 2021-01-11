package main

import (
	"errors"
	"github.com/denverquane/amongusdiscord/pkg/galactus_client"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/denverquane/amongusdiscord/locale"
	"github.com/denverquane/amongusdiscord/storage"

	"github.com/denverquane/amongusdiscord/discord"
)

var (
	version = "6.9.0"
	commit  = "none"
	date    = "unknown"
)

const DefaultURL = "http://localhost:8123"

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
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./"
	}

	logEntry := os.Getenv("DISABLE_LOG_FILE")
	if logEntry == "" {
		file, err := os.Create(path.Join(logPath, "logs.txt"))
		if err != nil {
			return err
		}
		mw := io.MultiWriter(os.Stdout, file)
		log.SetOutput(mw)
	}

	emojiGuildID := os.Getenv("EMOJI_GUILD_ID")

	log.Println(version + "-" + commit)

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}

	numShardsStr := os.Getenv("NUM_SHARDS")
	numShards, err := strconv.Atoi(numShardsStr)
	if err != nil {
		numShards = 1
	}

	shardIDStr := os.Getenv("SHARD_ID")
	shardID, err := strconv.Atoi(shardIDStr)
	if shardID >= numShards {
		return errors.New("you specified a shardID higher than or equal to the total number of shards")
	}
	if err != nil {
		shardID = 0
	}

	url := os.Getenv("HOST")
	if url == "" {
		log.Printf("[Info] No valid HOST provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
	}

	var redisClient discord.RedisInterface
	var storageInterface storage.StorageInterface

	if os.Getenv("AUTOMUTEUS_DEVELOPMENT") != "" {
		log.Println("Bot is running in DEVELOPMENT mode")
		redisClient.InitMock()
		storageInterface.InitMock()
	} else {
		log.Println("Bot is running in PRODUCTION mode")
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
			err = storageInterface.Init(storage.RedisParameters{
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
	}

	galactusAddr := os.Getenv("GALACTUS_ADDR")
	if galactusAddr == "" {
		return errors.New("no GALACTUS_ADDR specified; exiting")
	}

	galactusClient, err := galactus_client.NewGalactusClient(galactusAddr)
	if err != nil {
		log.Println("Error connecting to Galactus!")
		return err
	}

	locale.InitLang(os.Getenv("LOCALE_PATH"), os.Getenv("BOT_LANG"))

	psql := storage.PsqlInterface{}
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

	err = psql.Init(storage.ConstructPsqlConnectURL(pAddr, pUser, pPass))
	if err != nil {
		return err
	}

	if os.Getenv("AUTOMUTEUS_OFFICIAL") == "" {
		go psql.LoadAndExecFromFile("./storage/postgres.sql")
	}

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	bot := discord.MakeAndStartBot(version, commit, discordToken, url, emojiGuildID, numShards, shardID, &redisClient, &storageInterface, &psql, galactusClient, logPath)

	<-sc
	log.Printf("Received Sigterm or Kill signal. Bot will terminate in 1 second")
	time.Sleep(time.Second)

	bot.Close()
	return nil
}
