package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/docs"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/link.tmpl
var linkTemplateFileContents string

// Info is the /bot/info response. Unlike the Discord /info command, the API is
// not tied to a shard, so no shard fields are reported.
type Info struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	TotalGuilds int64  `json:"totalGuilds"`
	ActiveGames int64  `json:"activeGames"`
	TotalUsers  int64  `json:"totalUsers"`
	TotalGames  int64  `json:"totalGames"`
}

// Store is the shared data needed by the HTTP API. No Discord session is used.
type Store interface {
	Info(context.Context) (Info, error)
	GameState(context.Context, string, string) (json.RawMessage, error)
	RoomCode(context.Context, string) (string, error)
	Settings(context.Context, string) (*settings.GuildSettings, error)
	Premium(context.Context, string) (premium.PremiumRecord, error)
	Ping(context.Context) error
}

type Config struct {
	Version       string
	Commit        string
	ServerURL     string
	AdminPassword string
	CaptureHost   string
	Official      bool
}

func NewRouter(config Config, store Store) *gin.Engine {
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Title = "AutoMuteUs"
	docs.SwaggerInfo.Version = config.Version
	docs.SwaggerInfo.Description = "AutoMuteUs Bot API"
	var schemes []string
	host := config.ServerURL
	if host == "" {
		host = "http://localhost"
	}
	adminPassword := config.AdminPassword
	if adminPassword == "" {
		adminPassword = "automuteus"
	}
	if strings.HasPrefix(host, "http://") {
		schemes = append(schemes, "http")
		host = strings.Replace(host, "http://", "", 1)
	} else if strings.HasPrefix(host, "https://") {
		schemes = append(schemes, "https")
		host = strings.Replace(host, "https://", "", 1)
	}
	docs.SwaggerInfo.Host = host
	docs.SwaggerInfo.Schemes = schemes

	botGroup := r.Group("/bot")
	botGroup.GET("/info", handleGetInfo(store))
	botGroup.GET("/commands", handleGetCommands())

	// TODO in the future, I'd like this to receive a Discord Access Token
	// that way, any user that is logged in via Discord (not only through the web UI)
	// can get info about a game going on in a guild that they're a member of...
	gameGroup := r.Group("/game", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	gameGroup.GET("/state", handleGetGameState(store))
	gameGroup.GET("/roomcode", handleGetRoomCode(store))

	// TODO same as above, but we also need to check the User's permissions within the server in question
	// (aka if user is not a bot admin for a guild, they can't change that guild's settings)
	guildGroup := r.Group("/guild", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	guildGroup.GET("/settings", handleGetGuildSettings(store))
	guildGroup.GET("/premium", handleGetGuildPremium(store))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/open/link", handleGetOpenAmongUsCapture(config))
	r.GET("/live", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "API dependencies unavailable"})
			return
		}
		c.Status(http.StatusOK)
	})

	// TODO add endpoints for notable player information, like total games played, num wins, etc

	// TODO properly configure CORS -_-
	return r
}

// BotInfo godoc
// @Summary Get Bot Info
// @Description Get basic information about the bot
// @Tags bot
// @Accept json
// @Produce json
// @Success 200 {object} Info
// @Router /bot/info [get]
func handleGetInfo(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		info, err := store.Info(c.Request.Context())
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to load bot information"})
			return
		}
		c.JSON(http.StatusOK, info)
	}
}

// BotCommands godoc
// @Summary Get Bot Commands
// @Description Get all Discord commands that the bot implements
// @Tags bot
// @Accept json
// @Produce json
// @Success 200 {array} discordgo.ApplicationCommand
// @Router /bot/commands [get]
func handleGetCommands() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, command.All)
	}
}

// Open AmongUsCapture
// @Summary Get AmongUsCapture
// @Description Return html that open AmongUsCapture
// @Produce html
// @Success 200 {string} string "text/html"
// @Router /open/link [get]
func handleGetOpenAmongUsCapture(config Config) func(c *gin.Context) {
	return func(c *gin.Context) {
		connectCode := c.Query("connectCode")
		if len(connectCode) != 8 {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid connect code",
			})
			return
		}
		hyperlink, _, _ := capture.FormCaptureURL(config.CaptureHost, config.ServerURL, connectCode)
		t, err := template.New("template").Parse(linkTemplateFileContents)
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		err = t.Execute(c.Writer, map[string]string{
			"URL": hyperlink,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
	}
}

// GetGameState godoc
// @Summary Get Game State
// @Description Get the current state of a running game
// @Security BasicAuth
// @Tags game
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param connectCode query string true "Connect Code"
// @Success 200 {object} map[string]interface{} "Stored game state, using the bot's JSON representation"
// @Failure 400 {object} HttpError
// @Failure 500 {object} nil
// @Router /game/state [get]
func handleGetGameState(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		connectCode := c.Query("connectCode")
		if len(connectCode) != 8 {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid connect code",
			})
			return
		}
		state, err := store.GameState(c.Request.Context(), guildID, connectCode)
		if errors.Is(err, redis.Nil) {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "no game status found with those details",
			})
			return
		}

		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, state)
	}
}

// GetRoomCode godoc
// @Summary Get Room Code
// @Description Get the Among Us room code most recently reported by the capture client for a connect code
// @Security BasicAuth
// @Tags game
// @Accept json
// @Produce json
// @Param connectCode query string true "Connect Code"
// @Success 200 {object} RoomCodeResponse
// @Failure 400 {object} HttpError
// @Failure 404 {object} HttpError
// @Failure 500 {object} HttpError
// @Router /game/roomcode [get]
func handleGetRoomCode(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		connectCode := c.Query("connectCode")
		if len(connectCode) != 8 {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid connect code",
			})
			return
		}

		roomCode, err := store.RoomCode(c.Request.Context(), connectCode)
		if errors.Is(err, redis.Nil) {
			c.JSON(http.StatusNotFound, HttpError{
				StatusCode: http.StatusNotFound,
				Error:      "no room code found for that connect code",
			})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, RoomCodeResponse{
			ConnectCode: connectCode,
			RoomCode:    roomCode,
		})
	}
}

// GetGuildSettings godoc
// @Summary Get Guild Settings
// @Description Get the settings for a given guild. Guilds that never changed a setting get the defaults.
// @Security BasicAuth
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} settings.GuildSettings
// @Failure 400 {object} HttpError
// @Failure 503 {object} HttpError
// @Router /guild/settings [get]
func handleGetGuildSettings(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		settings, err := store.Settings(c.Request.Context(), guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{
				StatusCode: http.StatusServiceUnavailable,
				Error:      "Unable to load guild settings",
			})
			return
		}
		c.JSON(http.StatusOK, settings)
	}
}

// GetGuildPremium godoc
// @Summary Get Guild Premium
// @Description Get the premium status for a given guild
// @Security BasicAuth
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} premium.PremiumRecord
// @Failure 400 {object} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/premium [get]
func handleGetGuildPremium(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		record, err := store.Premium(c.Request.Context(), guildID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, record)
	}
}

type HttpError struct {
	StatusCode int
	Error      string
}

type RoomCodeResponse struct {
	ConnectCode string `json:"connectCode"`
	RoomCode    string `json:"roomCode"`
}
