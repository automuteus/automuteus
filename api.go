package main

import (
	"errors"
	"github.com/automuteus/automuteus/v8/api"
	"github.com/automuteus/automuteus/v8/pkg/redis"
	storage2 "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
)

func main() {
	if err := apiWrapper(); err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		return
	}

}

func apiWrapper() error {
	var isOfficial = os.Getenv("AUTOMUTEUS_OFFICIAL") != ""

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}

	var redisDriver redis.Driver
	var storageInterface storage2.StorageInterface

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASS")
	if redisAddr != "" {
		err := redisDriver.Init(storage2.RedisParameters{
			Addr:     redisAddr,
			Username: "",
			Password: redisPassword,
		})
		if err != nil {
			log.Println(err)
		}
		err = storageInterface.Init(storage2.RedisParameters{
			Addr:     redisAddr,
			Username: "",
			Password: redisPassword,
		})
		if err != nil {
			log.Println(err)
		}
	} else {
		return errors.New("no REDIS_ADDR specified; exiting")
	}

	psql := storage2.PsqlInterface{}
	pAddr := os.Getenv("POSTGRES_ADDR")
	if pAddr == "" {
		return errors.New("no POSTGRES_ADDR specified; exiting")
	}

	pUser := os.Getenv("POSTGRES_USER")
	if pUser == "" {
		return errors.New("no POSTGRES_USER specified; exiting")
	}

	pPass := os.Getenv("POSTGRES_PASS")
	if pPass == "" {
		return errors.New("no POSTGRES_PASS specified; exiting")
	}

	err := psql.Init(storage2.ConstructPsqlConnectURL(pAddr, pUser, pPass))
	if err != nil {
		return err
	}

	url := os.Getenv("API_SERVER_URL")
	if url == "" {
		url = "http://localhost"
	}
	adminPassword := os.Getenv("API_ADMIN_PASS")
	if adminPassword == "" {
		adminPassword = "automuteus"
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return err
	}

	a := api.NewApi(isOfficial, url, adminPassword, dg, redisDriver, storageInterface, psql)
	return a.StartServer("5000")
}
