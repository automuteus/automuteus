package main

import (
	"errors"
	"github.com/denverquane/amongusdiscord/capture"
	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"time"
)

func main() {
	err := mainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		log.Println("This window will automatically terminate in 30 seconds")
		time.Sleep(30 * time.Second)
		return
	}
}

func mainWrapper() error {
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
				return errors.New("unrecognized test argument! Please use 1-10 to test discussion player positions")
			}
			//testType = fmt.Sprintf("Player%dCapture", num)
			capture.TestNumberedDiscussCapture(capSettings, num-1)
		}
		if testType != "" {
			log.Printf("Testing `%s` mode, saving capture window to `%s.png`\n", testType, testType)
			log.Printf("OCR Results:\n%s\n", results)
		}
		return nil
	}

	captureResults := make(chan capture.GameState)

	//start the background worker that should be capturing the screen to monitor game state changes
	go capSettings.CaptureLoop(captureResults, debugLogs)

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
	discord.MakeAndStartBot(discordToken, discordGuild, discordChannel, captureResults, gameStartDelay, gameResumeDelay, discussStartDelay, discordMuteDelayMs)
	return nil
}
