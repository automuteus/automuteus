package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/denverquane/amongusdiscord/capture"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/joho/godotenv"
)

func main() {
	err := captureMainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		log.Println("This window will automatically terminate in 30 seconds")
		time.Sleep(30 * time.Second)
		return
	}
}

func captureMainWrapper() error {
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

	gameStateChannel := make(chan game.GameState)

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		return errors.New("empty SERVER_URL")
	}

	fmt.Println("Capture is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	go capture.RunClientSocketBroadcast(gameStateChannel, serverURL)

	//start the background worker that should be capturing the screen to monitor game state changes
	capSettings.CaptureLoop(gameStateChannel, debugLogs, sc)

	<-sc
	return nil
}
