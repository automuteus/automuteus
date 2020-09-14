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
	"strconv"
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
	monitorStr := os.Getenv("MONITOR")
	num := 0
	if monitorStr != "" {
		num, err := strconv.Atoi(monitorStr)
		if err != nil {
			log.Fatal(fmt.Sprintf("You provided an invalid display number for the MONITOR!\n"))
		} else {
			log.Printf("Running capture on display %d\n", num)
		}
	}
	xRes, yRes := 0, 0
	xResStr := os.Getenv("X_RESOLUTION")
	if xResStr != "" {
		x, err := strconv.Atoi(xResStr)
		if err != nil {
			log.Fatal("You provided a non-numerical value for the X resolution!")
		}
		xRes = x
	}

	yResStr := os.Getenv("Y_RESOLUTION")
	if yResStr != "" {
		y, err := strconv.Atoi(yResStr)
		if err != nil {
			log.Fatal("You provided a non-numerical value for the Y resolution!")
		}
		yRes = y
	}
	capSettings.xRes = float64(xRes)
	capSettings.yRes = float64(yRes)

	if capSettings.fullScreen {
		bounds := screenshot.GetDisplayBounds(num)
		if capSettings.xRes == 0 && capSettings.yRes == 0 {
			capSettings.xRes, capSettings.yRes = float64(bounds.Dx()), float64(bounds.Dy())
			log.Printf("Using auto-detected resolution: %dx%d\n", int(capSettings.xRes), int(capSettings.yRes))
		} else {
			log.Printf("Using .env-provided resolution: %dx%d\n", int(capSettings.xRes), int(capSettings.yRes))
		}
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
			gameStrings := genericCaptureAndOCR(settings.endingBounds, TempImageFilename)
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
			discussStrings := genericCaptureAndOCR(settings.discussBounds, TempImageFilename)
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
				endStrings := genericCaptureAndOCR(settings.endingBounds, TempImageFilename)
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
			endDiscussStrings := genericCaptureAndOCR(settings.discussBounds, TempImageFilename)
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

const xLeftStartScalar = 0.1738
const xRightStartScalar = 0.5144
const xWidthScalar = 0.0066

var yScalars = []float64{
	0.2187,  //1st row
	0.3458,  //2nd row
	0.47083, //3rd row
	0.5975,  //4th row
	0.7239,  //5th row
}

//height of the player image
const yHeightScalar = 0.00833

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

func genericCaptureAndOCR(bounds image.Rectangle, filename string) []string {
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(filename)
	if err != nil {
		log.Println("Encountered an issue making temp.png file! Maybe a permissions problem?")
		log.Println(err)
	}
	defer file.Close()
	err = png.Encode(file, img)
	if err != nil {
		log.Println("Error in encoding temp.png from png!")
		log.Println(err)
	}

	cmd := exec.Command(TESSERACT_PATH, filename, "stdout")
	//cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Println("Tesseract could not be ran with error:")
		log.Fatal(err)
	}
	finalString := strings.ReplaceAll(out.String(), "\r\n\f", "")
	finalString = strings.ReplaceAll(finalString, "\r\n", " ")
	finalString = strings.ToLower(finalString)
	return strings.Split(finalString, " ")
}

func genericCaptureAndRGBA(bounds image.Rectangle, filename string) RGBColor {
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		panic(err)
	}

	//TODO wont need to write out to file once done calibrating
	file, err := os.Create(filename)
	if err != nil {
		log.Println("Encountered an issue making temp.png file! Maybe a permissions problem?")
		log.Println(err)
	}
	defer file.Close()
	err = png.Encode(file, img)
	if err != nil {
		log.Println("Error in encoding temp.png from png!")
		log.Println(err)
	}

	bounds = img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	var rSum, gSum, bSum uint64

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rSum += uint64(r)
			gSum += uint64(g)
			bSum += uint64(b)
			//log.Println(r,g,b)
		}
	}
	totalPx := float64(width*height) * 257 //257 to convert back from a 32 bit value
	return RGBColor{
		r: uint32(float64(rSum) / totalPx),
		g: uint32(float64(gSum) / totalPx),
		b: uint32(float64(bSum) / totalPx),
	}
}

func TestDiscussCapture(settings CaptureSettings) []string {
	return genericCaptureAndOCR(settings.discussBounds, "discuss.png")
}

func TestEndingCapture(settings CaptureSettings) []string {
	return genericCaptureAndOCR(settings.endingBounds, "ending.png")
}

func TestNumberedDiscussCapture(settings CaptureSettings, num int) {
	color := genericCaptureAndRGBA(settings.playerNameBounds[num], fmt.Sprintf("Player%dCapture.png", num+1))
	log.Printf("R: %d, G: %d, B: %d\n", color.r, color.g, color.b)
}
