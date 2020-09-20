package main

import (
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/denverquane/amongusdiscord/discord"
	"github.com/joho/godotenv"
)

const DefaultPort = "8123"

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

	//TODO disabled move dead players for pre-release for a solid baseline of features
	//discordMoveDeadPlayersStr := os.Getenv("DISCORD_MOVE_DEAD_PLAYERS")
	discordMoveDeadPlayers := false
	//ret, err := strconv.ParseBool(discordMoveDeadPlayersStr)
	//if err == nil {
	//	log.Printf("Using DISCORD_MOVE_DEAD_PLAYERS %t\n", ret)
	//	discordMoveDeadPlayers = ret
	//} else {
	//	log.Printf("Problem parsing DISCORD_MOVE_DEAD_PLAYERS; using %t as default\n", discordMoveDeadPlayers)
	//}

	port := os.Getenv("SERVER_PORT")
	num, err := strconv.Atoi(port)
	if err != nil || num < 1000 || num > 9999 {
		log.Printf("Invalid or no particular SERVER_PORT provided. Defaulting to %s\n", DefaultPort)
		port = DefaultPort
	}

	//start the discord bot
	discord.MakeAndStartBot(discordToken, discordMoveDeadPlayers, port)
	return nil
}
