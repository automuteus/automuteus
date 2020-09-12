package main

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/kbinani/screenshot"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const xStartScalar = 0.19
const captureWidthScalar = 0.62

const yStartScalar = 0.1
const captureHeightScalar = 0.2

type CaptureSettings struct {
	fullScreen    bool
	xRes          int
	yRes          int
	discussBounds image.Rectangle
	endingBounds  image.Rectangle
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file; did you forget to rename 'sample.env' to '.env'?")
	}
	capSettings := CaptureSettings{}

	fullscreenStr := os.Getenv("FULLSCREEN")
	//explicitly default to fullscreen
	if fullscreenStr == "false" {
		capSettings.fullScreen = false
	} else {
		capSettings.fullScreen = true
	}

	if capSettings.fullScreen {
		bounds := screenshot.GetDisplayBounds(0)
		capSettings.xRes, capSettings.yRes = bounds.Dx(), bounds.Dy()
		startX, startY := int(math.Floor(float64(capSettings.xRes)*xStartScalar)), int(math.Floor(float64(capSettings.yRes)*yStartScalar))
		capSettings.endingBounds = image.Rectangle{
			Min: image.Point{
				X: startX,
				Y: startY,
			},
			Max: image.Point{
				X: startX + int(math.Floor(float64(capSettings.xRes)*captureWidthScalar)),
				Y: startY + int(math.Floor(float64(capSettings.yRes)*captureHeightScalar)),
			},
		}
	}
	log.Printf("Fullscreen: %v, Resolution: %d, %d\n", capSettings.fullScreen, capSettings.xRes, capSettings.yRes)
	go CaptureLoop(capSettings)

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		log.Fatal("No DISCORD_BOT_TOKEN provided! Exiting")
	}

	discordChannel := os.Getenv("DISCORD_CHANNEL_ID")
	if discordChannel == "" {
		log.Fatal("No DISCORD_CHANNEL_ID provided! Exiting")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func CaptureLoop(settings CaptureSettings) {
	for {
		start := time.Now()
		img, err := screenshot.CaptureRect(settings.endingBounds)
		if err != nil {
			panic(err)
		}
		file, _ := os.Create("temp.png")
		defer file.Close()
		png.Encode(file, img)

		cmd := exec.Command(`C:\Program Files\Tesseract-OCR\tesseract.exe`, "temp.png", "stdout")
		//cmd.Stdin = strings.NewReader("some input")
		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%q\n", out.String())

		log.Println(time.Now().Sub(start))
		time.Sleep(time.Millisecond * 500)
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
	// If the message is "ping" reply with "Pong!"
	if m.Content == "ping" {
		s.ChannelMessageSend(m.ChannelID, m.ChannelID)
	}

	// If the message is "pong" reply with "Ping!"
	if m.Content == "pong" {
		s.ChannelMessageSend(m.ChannelID, "Ping!")
	}
}
