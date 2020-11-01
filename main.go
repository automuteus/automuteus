package main

import (
	"errors"
	"fmt"
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
	"github.com/joho/godotenv"
)

var (
	version = "3.0.0"
	commit  = "none"
	date    = "unknown"
)

const DefaultURL = "http://localhost:8123"
const DefaultServicePort = "5000"
const DefaultSocketTimeoutSecs = 3600

func main() {
	//seed the rand generator (used for making connection codes)
	rand.Seed(time.Now().Unix())
	err := discordMainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		log.Println("This window will automatically terminate in 10 seconds")
		time.Sleep(10 * time.Second)
		return
	}
}

func discordMainWrapper() error {
	err := godotenv.Load("final.txt")
	if err != nil {
		err = godotenv.Load("config.txt")
		if err != nil && os.Getenv("DISCORD_BOT_TOKEN") == "" {
			log.Println("Can't open config file and missing DISCORD_BOT_TOKEN; creating config.txt for you to use for your config")
			f, err := os.Create("config.txt")
			if err != nil {
				log.Println("Issue creating sample config.txt")
				return err
			}
			_, err = f.WriteString(fmt.Sprintf("DISCORD_BOT_TOKEN=\nBOT_LANG=%s\n", locale.DefaultLang))
			f.Close()
		}
	}

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

	discordToken2 := os.Getenv("DISCORD_BOT_TOKEN_2")
	if discordToken2 != "" {
		log.Println("You provided a 2nd Discord Bot Token, so I'll try to use it")
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

	internalPort := os.Getenv("PORT")
	if internalPort == "" {
		log.Printf("[Info] No PORT provided. Defaulting to %s\n", discord.DefaultPort)
		internalPort = discord.DefaultPort
	} else {
		num, err := strconv.Atoi(internalPort)
		if err != nil || num > 65535 || (num < 1024 && num != 80 && num != 443) {
			return errors.New("invalid PORT (outside range [1024-65535] or 80/443) provided")
		}
	}

	servicePort := os.Getenv("SERVICE_PORT")
	if servicePort == "" {
		log.Printf("[Info] No SERVICE_PORT provided. Defaulting to %s\n", DefaultServicePort)
		servicePort = DefaultServicePort
	} else {
		num, err := strconv.Atoi(servicePort)
		if err != nil || num > 65535 || (num < 1024 && num != 80 && num != 443) {
			return errors.New("invalid SERVICE_PORT (outside range [1024-65535] or 80/443) provided")
		}
	}

	captureTimeout := DefaultSocketTimeoutSecs
	captureTimeoutStr := os.Getenv("CAPTURE_TIMEOUT")
	if captureTimeoutStr != "" {
		num, err := strconv.Atoi(captureTimeoutStr)
		if err != nil || num < 0 {
			return errors.New("invalid or non-numeric CAPTURE_TIMOUT provided")
		}
		captureTimeout = num
	}
	log.Printf("Using capture timeout of %d seconds\n", captureTimeout)

	var redisClient discord.RedisInterface
	var storageInterface storage.StorageInterface

	redisAddr := os.Getenv("REDIS_ADDRESS")
	redisPassword := os.Getenv("REDIS_PASSWORD")
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
		return errors.New("no Redis Address specified; exiting")
	}

	locale.InitLang(os.Getenv("BOT_LANG"))

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	bot := discord.MakeAndStartBot(version, commit, discordToken, discordToken2, url, internalPort, emojiGuildID, numShards, shardID, &redisClient, &storageInterface, logPath, captureTimeout)

	go discord.MessagesServer(servicePort, bot)

	<-sc
	bot.GracefulClose()
	log.Printf("Received Sigterm or Kill signal. Bot will terminate in 1 second")
	time.Sleep(time.Second)

	bot.Close()
	redisClient.Close()
	return nil
}
