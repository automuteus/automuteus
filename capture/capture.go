package capture

import (
	"bytes"
	"fmt"
	"github.com/kbinani/screenshot"
	"image"
	"image/png"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const DEBUG_DONT_TRANSITION = false
const TempImageFilename = "temp.png"

var TESSERACT_PATH = "C:\\Program Files\\Tesseract-OCR\\tesseract.exe"

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
	//really an integer type, but the computations are cleaner if we store as float
	xRes float64
	yRes float64

	discussBounds image.Rectangle
	endingBounds  image.Rectangle

	//all the player names in the discussion screen
	playerNameBounds []image.Rectangle
}

func (cap *CaptureSettings) ToString() string {
	buf := bytes.NewBuffer([]byte("Capture Settings:\n"))
	buf.WriteString(fmt.Sprintf("  Tesseract Path: %s\n", TESSERACT_PATH))
	buf.WriteString(fmt.Sprintf("  Fullscreen: %v\n", cap.fullScreen))
	buf.WriteString(fmt.Sprintf("  Resolution: %dx%d\n", int(cap.xRes), int(cap.yRes)))
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
	tesseractPathStr := os.Getenv("TESSERACT_PATH")
	if tesseractPathStr != "" {
		TESSERACT_PATH = tesseractPathStr
	}

	if capSettings.fullScreen {
		bounds := screenshot.GetDisplayBounds(0)
		capSettings.xRes, capSettings.yRes = float64(bounds.Dx()), float64(bounds.Dy())
		capSettings.endingBounds = generateCaptureWindow(capSettings.xRes, capSettings.yRes, endingXStartScalar, endingWidthScalar, endingYStartScalar, endingHeightScalar)
		capSettings.discussBounds = generateCaptureWindow(capSettings.xRes, capSettings.yRes, discussXStartScalar, discussWidthScalar, discussYStartScalar, discussHeightScalar)
		capSettings.generatePlayerNameBounds()
	}
	return capSettings
}

const CaptureLoopSleepMs = 500

func (settings *CaptureSettings) CaptureLoop(res chan<- GameState, debugLogs bool) {
	gameState := PREGAME
	for {
		//start := time.Now()
		switch gameState {
		//we only need to scan for the game start text
		case PREGAME:
			log.Println("Waiting for Game to begin...")
			gameStrings := genericCapture(settings.endingBounds, TempImageFilename)
			if debugLogs {
				log.Printf("OCR Results using Ending bounds:\n%s", gameStrings)
			}
			if intersects(gameStrings, BeginningStrings) {
				log.Println("Game has begun!")
				if !DEBUG_DONT_TRANSITION {
					res <- GAME
					gameState = GAME
				}
			}
		case GAME:
			log.Println("Waiting for Discussion or Game Over...")
			discussStrings := genericCapture(settings.discussBounds, TempImageFilename)
			if debugLogs {
				log.Printf("OCR Results using Discuss bounds:\n%s", discussStrings)
			}
			if intersects(discussStrings, DiscussionStrings) {
				log.Println("Discussion phase has begun!")
				if !DEBUG_DONT_TRANSITION {
					res <- DISCUSS
					gameState = DISCUSS
				}
			} else {
				//only check the end strings if we clearly havent begun a discussion
				endStrings := genericCapture(settings.endingBounds, TempImageFilename)
				if debugLogs {
					log.Printf("OCR Results using Ending bounds:\n%s", endStrings)
				}
				if intersects(endStrings, EndgameStrings) {
					log.Println("Game is over!")
					if !DEBUG_DONT_TRANSITION {
						res <- PREGAME
						gameState = PREGAME
					}
				}
			}
		case DISCUSS:
			log.Println("Waiting for discussion to end...")
			endDiscussStrings := genericCapture(settings.discussBounds, TempImageFilename)
			if debugLogs {
				log.Printf("OCR Results using Discuss bounds:\n%s", endDiscussStrings)
			}
			if intersects(endDiscussStrings, EndDiscussionStrings) {
				log.Println("Discussion is over!")
				if !DEBUG_DONT_TRANSITION {
					res <- GAME
					gameState = GAME
				}
			}
			//else {
			//	//this is an edge case, but the game can end if someone leaves during discussion
			//	//only check the end strings if we clearly havent begun a discussion
			//	endStrings := genericCapture(settings.endingBounds, TempImageFilename)
			//	if debugLogs {
			//		log.Printf("OCR Results using Ending bounds:\n%s", endStrings)
			//	}
			//	if intersects(endStrings, EndgameStrings) {
			//		log.Println("Game is over!")
			//		if !DEBUG_DONT_TRANSITION {
			//			res <- PREGAME
			//			gameState = PREGAME
			//		}
			//	}
			//}
		}

		//if debugLogs {
		//	log.Println(fmt.Sprintf("Took %s to capture and process screen", time.Now().Sub(start)))
		//	log.Println(fmt.Sprintf("Sleeping for %dms", CaptureLoopSleepMs))
		//}

		time.Sleep(time.Millisecond * CaptureLoopSleepMs)
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

const xLeftStartScalar = 0.205
const xRightStartScalar = 0.547
const xWidthScalar = 0.18

var yScalars = []float64{
	0.198, //1st row
	0.326, //2nd row
	0.451, //3rd row
	0.579, //4th row
	0.705, //5th row
}

const yHeightScalar = 0.06

func (settings *CaptureSettings) generatePlayerNameBounds() {
	settings.playerNameBounds = make([]image.Rectangle, 10)
	for i := 0; i < 10; i += 2 {
		settings.playerNameBounds[i] = generateCaptureWindow(settings.xRes, settings.yRes, xLeftStartScalar, xWidthScalar, yScalars[i/2], yHeightScalar)
		settings.playerNameBounds[i+1] = generateCaptureWindow(settings.xRes, settings.yRes, xRightStartScalar, xWidthScalar, yScalars[i/2], yHeightScalar)
	}
}

func generateCaptureWindow(xRes, yRes float64, xScalar, widthScalar, yScalar, heightScalar float64) image.Rectangle {
	startX, startY := xRes*xScalar, yRes*yScalar
	return image.Rectangle{
		Min: image.Point{
			X: int(startX),
			Y: int(startY),
		},
		Max: image.Point{
			X: int(startX + (xRes * widthScalar)),
			Y: int(startY + (yRes * heightScalar)),
		},
	}
}

//any 1 or more spaces
//var Spaces = regexp.MustCompile(`\s+`)

func genericCapture(bounds image.Rectangle, filename string) []string {
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		panic(err)
	}

	file, _ := os.Create(filename)
	defer file.Close()
	png.Encode(file, img)

	cmd := exec.Command(TESSERACT_PATH, filename, "stdout")
	//cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	finalString := strings.ReplaceAll(out.String(), "\r\n\f", "")
	finalString = strings.ReplaceAll(finalString, "\r\n", " ")
	finalString = strings.ToLower(finalString)
	return strings.Split(finalString, " ")
}

func TestDiscussCapture(settings CaptureSettings) []string {
	return genericCapture(settings.discussBounds, "discuss.png")
}

func TestEndingCapture(settings CaptureSettings) []string {
	return genericCapture(settings.endingBounds, "ending.png")
}

func TestNumberedDiscussCapture(settings CaptureSettings, num int) []string {
	return genericCapture(settings.playerNameBounds[num], fmt.Sprintf("Player%dCapture.png", num+1))
}
