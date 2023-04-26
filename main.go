package main

import (
	"log"
	"math/rand"
	"os"
	"time"
)

func main() {
	// seed the rand generator (used for making connection codes)
	rand.Seed(time.Now().Unix())

	if os.Getenv("AUTOMUTEUS_API") != "" {
		if err := apiWrapper(); err != nil {
			log.Println("Program exited with the following error:")
			log.Println(err)
			return
		}
	} else {
		if err := discordMainWrapper(); err != nil {
			log.Println("Program exited with the following error:")
			log.Println(err)
			return
		}
	}
}
