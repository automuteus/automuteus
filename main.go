package main

import (
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/discord/command"
	"github.com/automuteus/utils/pkg/locale"
	storage2 "github.com/automuteus/utils/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/automuteus/automuteus/storage"

	"github.com/automuteus/automuteus/discord"
)

var (
	version = "7.0.0"
	commit  = "none"
	date    = "unknown"
)

const DefaultURL = "http://localhost:8123"

type registeredCommand struct {
	GuildID            string
	ApplicationCommand *discordgo.ApplicationCommand
}

func main() {
	// seed the rand generator (used for making connection codes)
	rand.Seed(time.Now().Unix())
	err := discordMainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		return
	}
}

func discordMainWrapper() error {
	var isOfficial = os.Getenv("AUTOMUTEUS_OFFICIAL") != ""

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./"
	}

	logEntry := os.Getenv("DISABLE_LOG_FILE")
	if logEntry == "" {
		file, err := os.Create(path.Join(logPath, "logs.txt"))
		if err != nil {
			return err
		}
		mw := io.MultiWriter(os.Stdout, file)
		log.SetOutput(mw)
	}

	emojiGuildID := os.Getenv("EMOJI_GUILD_ID")

	log.Println(version + "-" + commit)

	if os.Getenv("WORKER_BOT_TOKENS") != "" {
		log.Println("WORKER_BOT_TOKENS is now a variable used by Galactus, not AutoMuteUs!")
		log.Fatal("Move WORKER_BOT_TOKENS to Galactus' config, then try again")
	}

	numShardsStr := os.Getenv("NUM_SHARDS")
	numShards, err := strconv.Atoi(numShardsStr)
	if err != nil {
		log.Println("No NUM_SHARDS specified; defaulting to 1")
		numShards = 1
	}

	shardIDStr := os.Getenv("SHARD_ID")
	if shardIDStr != "" {
		return errors.New("SHARD_ID is no longer supported! Please use SHARD_RANGE instead")
	}

	var shardRange shardRange
	shardRangeStr := os.Getenv("SHARD_RANGE")
	if shardRangeStr == "" {
		log.Println("No SHARD_RANGE specified, defaulting to 0,0")
		shardRange = defaultShardRange()
	} else {
		shardRange, err = parseShardRange(os.Getenv("SHARD_RANGE"), numShards)
		if err != nil {
			return err
		}
	}

	url := os.Getenv("HOST")
	if url == "" {
		log.Printf("[Info] No valid HOST provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
	}

	var redisClient discord.RedisInterface
	var storageInterface storage.StorageInterface

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASS")
	if redisAddr != "" {
		err := redisClient.Init(storage.RedisParameters{
			Addr:     redisAddr,
			Username: "",
			Password: redisPassword,
		})
		if err != nil {
			log.Println(err)
		}
		err = storageInterface.Init(storage.RedisParameters{
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

	galactusAddr := os.Getenv("GALACTUS_ADDR")
	if galactusAddr == "" {
		return errors.New("no GALACTUS_ADDR specified; exiting")
	}

	galactusClient, err := discord.NewGalactusClient(galactusAddr)
	if err != nil {
		log.Println("Error connecting to Galactus!")
		return err
	}

	locale.InitLang(os.Getenv("LOCALE_PATH"), os.Getenv("BOT_LANG"))

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

	err = psql.Init(storage2.ConstructPsqlConnectURL(pAddr, pUser, pPass))
	if err != nil {
		return err
	}

	if !isOfficial {
		go func() {
			err := psql.LoadAndExecFromFile("./storage/postgres.sql")
			if err != nil {
				log.Println("Exiting with fatal error when attempting to execute postgres.sql:")
				log.Fatal(err)
			}
		}()
	}

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	bots := make([]*discord.Bot, shardRange.max-shardRange.min)
	var i int
	for shard := shardRange.min; shard < shardRange.max; shard++ {
		bots[i] = discord.MakeAndStartBot(version, commit, discordToken, url, emojiGuildID, numShards, shard, &redisClient, &storageInterface, &psql, galactusClient, logPath)
		if bots[i] == nil {
			log.Fatalf("bot %d failed to initialize; did you provide a valid Discord Bot Token?", shard)
		}
		i++
	}

	// empty string entry = global
	slashCommandGuildIds := []string{""}
	slashCommandGuildIdStr := strings.ReplaceAll(os.Getenv("SLASH_COMMAND_GUILD_IDS"), " ", "")
	if slashCommandGuildIdStr != "" {
		slashCommandGuildIds = strings.Split(slashCommandGuildIdStr, ",")
	}

	// only register commands if we're not the official bot, OR we're the first shard
	var registeredCommands []registeredCommand
	if !isOfficial || shardRange.isFirstShard() {
		for _, guild := range slashCommandGuildIds {
			for _, v := range command.All {
				if guild == "" {
					log.Printf("Registering command %s GLOBALLY\n", v.Name)
				} else {
					log.Printf("Registering command %s in guild %s\n", v.Name, guild)
				}

				id, err := bots[0].PrimarySession.ApplicationCommandCreate(bots[0].PrimarySession.State.User.ID, guild, v)
				if err != nil {
					log.Panicf("Cannot create command: %v", err)
				} else {
					registeredCommands = append(registeredCommands, registeredCommand{
						GuildID:            guild,
						ApplicationCommand: id,
					})
				}
			}
		}
		log.Println("Finishing registering all commands!")
	}

	<-sc
	log.Printf("Received Sigterm or Kill signal. Bot will terminate in 1 second")
	time.Sleep(time.Second)

	// only delete the slash commands if we're not the official bot, and the first shard
	if !isOfficial && shardRange.isFirstShard() {
		log.Println("Deleting slash commands")
		for _, v := range registeredCommands {
			if v.GuildID == "" {
				log.Printf("Deleting command %s GLOBALLY\n", v.ApplicationCommand.Name)
			} else {
				log.Printf("Deleting command %s on guild %s\n", v.ApplicationCommand.Name, v.GuildID)
			}
			err = bots[0].PrimarySession.ApplicationCommandDelete(v.ApplicationCommand.ApplicationID, v.GuildID, v.ApplicationCommand.ID)
			if err != nil {
				log.Println(err)
			}
		}
		log.Println("Finished deleting all commands")
	}

	for _, v := range bots {
		v.Close()
	}
	return nil
}

type shardRange struct {
	min, max int
}

func defaultShardRange() shardRange {
	return shardRange{
		min: 0,
		max: 0,
	}
}

// IsFirstShard indicates if we're running in the (default) single shard mode
func (sr shardRange) isFirstShard() bool {
	return sr.min == 0 && sr.max == 0
}

func parseShardRange(str string, maxShards int) (shardRange, error) {
	var sRange shardRange
	var err error
	var min, max uint64

	tokens := strings.Split(strings.ReplaceAll(str, " ", ""), ",")
	if len(tokens) != 2 {
		return sRange, errors.New("error parsing shard range string: \"" + str + "\"; expected 2 uints separated by ,")
	}
	min, err = strconv.ParseUint(tokens[0], 10, 64)
	if err != nil {
		return sRange, err
	}
	max, err = strconv.ParseUint(tokens[1], 10, 64)
	if err != nil {
		return sRange, err
	}
	sRange.min = int(min)
	sRange.max = int(max)
	if sRange.min > sRange.max {
		return sRange, fmt.Errorf("shard range min: %d is greater than shard range max: %d", sRange.min, sRange.max)
	}
	if sRange.max >= maxShards {
		return sRange, fmt.Errorf("shard range max: %d is greater or equal to the total number of shards: %d", sRange.max, max)
	}
	return sRange, nil
}
