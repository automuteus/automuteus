package capture

import (
	"bytes"
	"fmt"
	"github.com/kbinani/screenshot"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"
)

const TempImageFilename = "temp.png"

const discussXStartScalar = 0.28
const discussWidthScalar = 0.4
const discussYStartScalar = 0.09
const discussHeightScalar = 0.1

const endingXStartScalar = 0.17
const endingWidthScalar = 0.65
const endingYStartScalar = 0.1
const endingHeightScalar = 0.2

type GameState int

const (
	PREGAME GameState = 0
	GAME    GameState = 1
	DISCUSS GameState = 2
	KILL    GameState = 10
)

type CaptureSettings struct {
	fullScreen bool
	xRes       int
	yRes       int

	discussBounds image.Rectangle
	endingBounds  image.Rectangle
}

func (cap *CaptureSettings) ToString() string {
	buf := bytes.NewBuffer([]byte("Capture Settings:\n"))
	buf.WriteString(fmt.Sprintf("  Fullscreen: %v\n", cap.fullScreen))
	buf.WriteString(fmt.Sprintf("  Resolution: %dx%d\n", cap.xRes, cap.yRes))
	disc := cap.discussBounds
	buf.WriteString(fmt.Sprintf("  Discussion Bounds: [%d,%d]-[%d, %d]\n", disc.Min.X, disc.Min.Y, disc.Max.X, disc.Max.Y))
	end := cap.endingBounds
	buf.WriteString(fmt.Sprintf("  Ending Bounds: [%d,%d]-[%d, %d]\n", end.Min.X, end.Min.Y, end.Max.X, end.Max.Y))
	return buf.String()
}

func MakeSettingsFromEnv() CaptureSettings {
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
		startX, startY := int(math.Floor(float64(capSettings.xRes)*endingXStartScalar)), int(math.Floor(float64(capSettings.yRes)*endingYStartScalar))
		capSettings.endingBounds = image.Rectangle{
			Min: image.Point{
				X: startX,
				Y: startY,
			},
			Max: image.Point{
				X: startX + int(math.Floor(float64(capSettings.xRes)*endingWidthScalar)),
				Y: startY + int(math.Floor(float64(capSettings.yRes)*endingHeightScalar)),
			},
		}
		startX, startY = int(math.Floor(float64(capSettings.xRes)*discussXStartScalar)), int(math.Floor(float64(capSettings.yRes)*discussYStartScalar))
		capSettings.discussBounds = image.Rectangle{
			Min: image.Point{
				X: startX,
				Y: startY,
			},
			Max: image.Point{
				X: startX + int(math.Floor(float64(capSettings.xRes)*discussWidthScalar)),
				Y: startY + int(math.Floor(float64(capSettings.yRes)*discussHeightScalar)),
			},
		}
	}
	return capSettings
}

func (settings *CaptureSettings) CaptureLoop(res chan<- GameState) {
	gameState := PREGAME
	for {
		start := time.Now()
		switch gameState {
		//we only need to scan for the game start text
		case PREGAME:
			gameStrings := genericCapture(settings.endingBounds)
			if intersects(gameStrings, BeginningStrings) {
				log.Println("Game has begun!")
				res <- GAME
				gameState = GAME
			}
		case GAME:
			discussStrings := genericCapture(settings.discussBounds)
			if intersects(discussStrings, DiscussionStrings) {
				log.Println("Discussion phase has begun!")
				res <- DISCUSS
				gameState = DISCUSS
			} else {
				//only check the end strings if we clearly havent begun a discussion
				endStrings := genericCapture(settings.endingBounds)
				if intersects(endStrings, EndgameStrings) {
					log.Println("Game is over!")
					res <- PREGAME
					gameState = PREGAME
				}
			}
		case DISCUSS:
			endDiscussStrings := genericCapture(settings.discussBounds)
			if intersects(endDiscussStrings, EndDiscussionStrings) {
				log.Println("Discussion is over!")
				res <- GAME
				gameState = GAME
			}
		}

		log.Println(time.Now().Sub(start))
		time.Sleep(time.Millisecond * 500)
	}
}

func intersects(a, b []string) bool {
	for _, v := range a {
		for _, vv := range b {
			if v == vv {
				return true
			}
		}
	}
	return false
}

func genericCapture(bounds image.Rectangle) []string {
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		panic(err)
	}
	file, _ := os.Create(TempImageFilename)
	defer file.Close()
	png.Encode(file, img)

	cmd := exec.Command(`C:\Program Files\Tesseract-OCR\tesseract.exe`, TempImageFilename, "stdout")
	//cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	finalString := strings.ReplaceAll(out.String(), "\r\n", " ")
	finalString = strings.ToLower(finalString)
	return strings.Split(finalString, " ")
}
