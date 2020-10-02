package main

import (
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
)

const VERSION = "2.2.2-Prerelease"

const DefaultPort = "8123"

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
	err := godotenv.Load("final.env")
	if err != nil {
		err = godotenv.Load("final.txt")
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

	port := os.Getenv("SERVER_PORT")
	num, err := strconv.Atoi(port)
	if err != nil || num < 1024 || num > 65535 {
		log.Printf("Invalid or no particular SERVER_PORT (range [1024-65535]) provided. Defaulting to %s\n", DefaultPort)
		port = DefaultPort
	}

	//start the discord bot
	discord.MakeAndStartBot(VERSION, discordToken, port, emojiGuildID, numShards, shardID)
	return nil
}
