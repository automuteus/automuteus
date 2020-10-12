package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/denverquane/amongusdiscord/storage"

	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
)

const VERSION = "2.3.2-Prerelease"

//TODO if running in shard mode, we don't want to use the default port. Each shard should prob run on their own port
const DefaultPort = "8123"
const DefaultURL = "http://localhost"

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
	err := godotenv.Load("config.txt")
	if err != nil {
		err = godotenv.Load("final.txt")
		if err != nil {
			log.Println("Can't open config file, hopefully you're running in docker and have provided the DISCORD_BOT_TOKEN...")
			f, err := os.Create("config.txt")
			if err != nil {
				log.Println("Issue creating sample config.txt")
				return err
			}
			_, err = f.WriteString("DISCORD_BOT_TOKEN = \n")
			f.Close()
		}
	}

	logEntry := os.Getenv("DISABLE_LOG_FILE")
	if logEntry == "" {
		file, err := os.Create("logs.txt")
		if err != nil {
			return err
		}
		mw := io.MultiWriter(os.Stdout, file)
		log.SetOutput(mw)
	}

	emojiGuildID := os.Getenv("EMOJI_GUILD_ID")

	log.Println(VERSION)

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

	portStr := os.Getenv("PORT")
	port := ""

	num, err := strconv.Atoi(portStr)

	if err != nil || num < 1024 || num > 65535 {
		log.Printf("[Info] Invalid or no particular PORT (range [1024-65535]) provided. Defaulting to %s\n", DefaultPort)
		port = DefaultPort
	} else {
		port = portStr
	}

	url := os.Getenv("SERVER_URL")
	if url == "" {
		log.Printf("[Info] No valid SERVER_URL provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
	}

	extPort := os.Getenv("EXT_PORT")
	if extPort == "" {
		log.Print("[Info] No EXT_PORT provided. Defaulting to PORT\n")
	} else if extPort == "protocol" {
		log.Print("[Info] EXT_PORT set to protocol. Not adding port to url\n")
	} else {
		num, err := strconv.Atoi(extPort)
		if err != nil || num > 65535 || (num < 1024 && num != 80 && num != 443) {
			return errors.New("invalid EXT_PORT (range [1024-65535]) provided")
		}
	}

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

	bot := discord.MakeAndStartBot(VERSION, discordToken, discordToken2, url, port, extPort, emojiGuildID, numShards, shardID, storageClient)

	go discord.MessagesServer("5000", bot)

	<-sc
	bot.Close()
	storageClient.Close()
	return nil
}
