package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/denverquane/amongusdiscord/storage"

	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
)

var (
	version = "2.4.0"
	commit  = "none"
	date    = "unknown"
)

const DefaultURL = "http://localhost:8123"
const DefaultServicePort = "5000"
const DefaultSocketTimeoutSecs = 3600

func main() {
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
			_, err = f.WriteString("DISCORD_BOT_TOKEN=\n")
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
	}
	log.Printf("Using capture timeout of %d seconds\n", captureTimeout)

	var storageClient storage.StorageInterface
	dbSuccess := false

	authPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	projectID := os.Getenv("FIRESTORE_PROJECT_ID")
	if authPath != "" && projectID != "" {
		log.Println("GOOGLE_APPLICATION_CREDENTIALS variable is set; attempting to use Firestore as the Storage Driver")
		storageClient = &storage.FirestoreDriver{}
		err = storageClient.Init(projectID)
		if err != nil {
			log.Printf("Failed to create Firestore client with error: %s", err)
		} else {
			dbSuccess = true
			log.Println("Success in initializing Firestore client as the Storage Driver")
		}
	}

	if !dbSuccess {
		storageClient = &storage.FilesystemDriver{}
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "./"
		}
		log.Printf("Using %s as the base path for config", configPath)
		err := storageClient.Init(configPath)
		if err != nil {
			log.Fatalf("Failed to create Filesystem Storage Driver with error: %s", err)
		}
		log.Println("Success in initializing the local Filesystem as the Storage Driver")
	}

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	bot := discord.MakeAndStartBot(version+"-"+commit, discordToken, discordToken2, url, internalPort, emojiGuildID, numShards, shardID, storageClient, logPath, captureTimeout)

	go discord.MessagesServer(servicePort, bot)

	<-sc
	bot.GracefulClose(5, "**Bot has been terminated, so I'm killing your game in 5 seconds!**")
	log.Printf("Received Sigterm or Kill signal. Bot will terminate in 5 seconds")
	time.Sleep(time.Second * time.Duration(5))

	bot.Close()
	storageClient.Close()
	return nil
}
