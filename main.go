package main

import (
	"github.com/denverquane/amongusdiscord/capture"
	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file; did you forget to rename 'sample.env' to '.env'?")
	}

	capSettings := capture.MakeSettingsFromEnv()

	log.Println(capSettings.ToString())

	captureResults := make(chan capture.GameState)

	//start the background worker that should be capturing the screen to monitor game state changes
	//go capSettings.CaptureLoop(captureResults)

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		log.Fatal("No DISCORD_BOT_TOKEN provided! Exiting")
	}

	discordGuild := os.Getenv("DISCORD_GUILD_ID")
	if discordGuild == "" {
		log.Fatal("No DISCORD_GUILD_ID provided! Exiting")
	}

	discordChannel := os.Getenv("DISCORD_CHANNEL_ID")
	if discordChannel == "" {
		log.Println("No DISCORD_CHANNEL_ID provided, assuming commands from any channel are equally valid")
	}

	//start the discord bot
	discord.MakeAndStartBot(discordToken, discordGuild, discordChannel, captureResults)
}
