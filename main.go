package main

import (
	"fmt"
	"github.com/denverquane/amongusdiscord/capture"
	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
)

func main() {
	err := godotenv.Load("final.env")
	if err != nil {
		log.Fatal("Error loading final.env file; did you forget to rename 'sample.env' to 'final.env'?")
	}

	debugLogsStr := os.Getenv("DEBUG_LOGS")
	debugLogs := debugLogsStr == "true"

	capSettings := capture.MakeSettingsFromEnv()

	log.Println(capSettings.ToString())

	if len(os.Args) > 1 {
		testType := ""
		var results []string

		if os.Args[1] == "discuss" || os.Args[1] == "d" {
			testType = "discuss"
			results = capture.TestDiscussCapture(capSettings)
		} else if os.Args[1] == "ending" || os.Args[1] == "end" || os.Args[1] == "e" {
			testType = "ending"
			results = capture.TestEndingCapture(capSettings)
		} else {
			num, err := strconv.Atoi(os.Args[1])
			if err != nil {
				log.Fatal("Unrecognized test argument! Please use 1-10 to test discussion player positions")
			}
			testType = fmt.Sprintf("Player%dCapture", num)
			results = capture.TestNumberedDiscussCapture(capSettings, num-1)
		}
		if testType != "" {
			log.Printf("Testing `%s` mode, saving capture window to `%s.png`\n", testType, testType)
			log.Printf("OCR Results:\n%s\n", results)
		}
		return
	}

	captureResults := make(chan capture.GameState)

	//start the background worker that should be capturing the screen to monitor game state changes
	go capSettings.CaptureLoop(captureResults, debugLogs)

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

	gameResumeDelayStr := os.Getenv("GAME_RESUME_DELAY")
	gameResumeDelay := 5
	num, err := strconv.Atoi(gameResumeDelayStr)
	if err == nil {
		log.Printf("Using GAME_RESUME_DELAY of %d seconds\n", num)
		gameResumeDelay = num
	} else {
		log.Printf("Error parsing GAME_RESUME_DELAY; using %d seconds as default\n", gameResumeDelay)
	}

	discussStartDelayStr := os.Getenv("DISCUSS_START_DELAY")
	discussStartDelay := 2
	num, err = strconv.Atoi(discussStartDelayStr)
	if err == nil {
		log.Printf("Using DISCUSS_START_DELAY of %d seconds\n", num)
		discussStartDelay = num
	} else {
		log.Printf("Error parsing DISCUSS_START_DELAY; using %d seconds as default\n", discussStartDelay)
	}

	//start the discord bot
	discord.MakeAndStartBot(discordToken, discordGuild, discordChannel, captureResults, gameResumeDelay, discussStartDelay)
}
