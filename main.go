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

const VERSION = "2.2.1-Prerelease"

const DefaultPort = "8123"
const DefaultURL = "localhost"

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

	port := os.Getenv("SERVER_PORT")
	num, err := strconv.Atoi(port)
	if err != nil || num < 1000 || num > 9999 {
		log.Printf("[This is not an error] No valid SERVER_PORT provided. Defaulting to %s\n", DefaultPort)
		port = DefaultPort
	}

	url := os.Getenv("SERVER_URL")
	if url == "" {
		log.Printf("[This is not an error] No valid SERVER_URL provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
	}

	//start the discord bot
	discord.MakeAndStartBot(discordToken, url, port, emojiGuildID)
	return nil
}
