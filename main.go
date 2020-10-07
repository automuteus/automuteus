package main

import (
	"errors"
	"github.com/denverquane/amongusdiscord/storage"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
)

const VERSION = "2.5.0-Prerelease"

const DefaultPort = "8123"
const DefaultURL = "http://localhost"

const ConfigBasePath = "./"

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
		err = godotenv.Load("final.env")
		if err != nil {
			log.Println("Can't open env file, hopefully you're running in docker and have provided the DISCORD_BOT_TOKEN...")
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
		numShards = 0
	}
	shardIDStr := os.Getenv("SHARD_ID")
	shardID, err := strconv.Atoi(shardIDStr)
	if err != nil {
		shardID = -1
	}

	port := os.Getenv("PORT")
	num, err := strconv.Atoi(port)

	if err != nil || num < 1024 || num > 65535 {
		log.Printf("[Info] Invalid or no particular PORT (range [1024-65535]) provided. Defaulting to %s\n", DefaultPort)
		port = DefaultPort
	}

	url := os.Getenv("SERVER_URL")
	if url == "" {
		log.Printf("[Info] No valid SERVER_URL provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
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
		err := storageClient.Init(ConfigBasePath)
		if err != nil {
			log.Fatalf("Failed to create Filesystem Storage Driver with error: %s", err)
		}
		log.Println("Success in initializing the local Filesystem as the Storage Driver")
	}

	//start the discord bot
	discord.MakeAndStartBot(VERSION, discordToken, discordToken2, url, port, emojiGuildID, numShards, shardID, storageClient)
	return nil
}
