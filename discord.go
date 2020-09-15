package main

import (
	"errors"
	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"time"
)

func main() {
	err := discordMainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		log.Println("This window will automatically terminate in 30 seconds")
		time.Sleep(30 * time.Second)
		return
	}
}

func discordMainWrapper() error {
	err := godotenv.Load("final.env")
	if err != nil {
		err = godotenv.Load("final.txt")
		if err != nil {
			err = godotenv.Load("final.env.txt")
			if err != nil {
				return errors.New("error loading environment file; you need a file named final.env, final.txt, or final.env.txt")
			}
		}
	}

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}

	discordGuild := os.Getenv("DISCORD_GUILD_ID")
	if discordGuild == "" {
		return errors.New("no DISCORD_GUILD_ID provided")
	}

	discordChannel := os.Getenv("DISCORD_CHANNEL_ID")
	if discordChannel == "" {
		return errors.New("no DISCORD_CHANNEL_ID provided")
	}

	gameStartDelayStr := os.Getenv("GAME_START_DELAY")
	gameStartDelay := 4
	num, err := strconv.Atoi(gameStartDelayStr)
	if err == nil {
		log.Printf("Using GAME_START_DELAY of %d seconds\n", num)
		gameStartDelay = num
	} else {
		log.Printf("Problem parsing GAME_RESUME_DELAY; using %d seconds as default\n", gameStartDelay)
	}

	gameResumeDelayStr := os.Getenv("GAME_RESUME_DELAY")
	gameResumeDelay := 7
	num, err = strconv.Atoi(gameResumeDelayStr)
	if err == nil {
		log.Printf("Using GAME_RESUME_DELAY of %d seconds\n", num)
		gameResumeDelay = num
	} else {
		log.Printf("Problem parsing GAME_RESUME_DELAY; using %d seconds as default\n", gameResumeDelay)
	}

	discussStartDelayStr := os.Getenv("DISCUSS_START_DELAY")
	discussStartDelay := 0
	num, err = strconv.Atoi(discussStartDelayStr)
	if err == nil {
		log.Printf("Using DISCUSS_START_DELAY of %d seconds\n", num)
		discussStartDelay = num
	} else {
		log.Printf("Problem parsing DISCUSS_START_DELAY; using %d seconds as default\n", discussStartDelay)
	}

	discordMuteDelayMsStr := os.Getenv("DISCORD_API_MUTE_DELAY_MS")
	discordMuteDelayMs := 300
	num, err = strconv.Atoi(discordMuteDelayMsStr)
	if err == nil {
		log.Printf("Using DISCORD_API_MUTE_DELAY_MS of %dms\n", num)
		discordMuteDelayMs = num
	} else {
		log.Printf("Problem parsing DISCORD_API_MUTE_DELAY_MS; using %dms as default\n", discordMuteDelayMs)
	}

	//start the discord bot
	discord.MakeAndStartBot(discordToken, discordGuild, discordChannel, gameStartDelay, gameResumeDelay, discussStartDelay, discordMuteDelayMs)
	return nil

}
