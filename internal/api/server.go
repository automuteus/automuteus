package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/docs"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/notice"
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
	ActiveNotice(context.Context) (*notice.Notice, error)
	RaiseNotice(context.Context, notice.Notice) error
	ClearNotice(context.Context) error
}

type Config struct {
	// GuildVerifier is injectable for tests; nil uses Discord HTTPS endpoints.
	GuildVerifier GuildVerifier
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

	verifier := config.GuildVerifier
	if verifier == nil {
		verifier = newDiscordVerifier()
	}
	gameGroup := r.Group("/game", guildAuthentication(config, verifier, ReadGame))
	gameGroup.GET("/state", handleGetGameState(store))
	gameGroup.GET("/roomcode", handleGetRoomCode(store))
	guildGroup := r.Group("/guild")
	guildGroup.GET("/settings", guildAuthentication(config, verifier, ReadSettings), handleGetGuildSettings(store))
	guildGroup.GET("/premium", guildAuthentication(config, verifier, ReadPremium), handleGetGuildPremium(store))

	// Platform notices: warn players about maintenance, or (critical) end every running game. Raising and clearing
	// notices requires an explicitly configured admin password; the default password is refused.
	adminGroup := r.Group("/admin", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	adminGroup.GET("/notice", handleGetNotice(store))
	adminGroup.POST("/notice", requireConfiguredPassword(config), handlePostNotice(store))
	adminGroup.DELETE("/notice", requireConfiguredPassword(config), handleDeleteNotice(store))

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
// @Description Get the current state of a running game. Bearer callers receive MemberGameState; platform Basic Auth receives the stored document.
// @Security BasicAuth
// @Security DiscordBearer
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
		if c.GetBool(memberRequestKey) {
			view, err := memberGameState(state, guildID, connectCode)
			if err != nil {
				c.JSON(http.StatusInternalServerError, HttpError{StatusCode: 500, Error: "Invalid stored game state"})
				return
			}
			c.JSON(http.StatusOK, view)
			return
		}
		c.JSON(http.StatusOK, state)
	}
}

// GetRoomCode godoc
// @Summary Get Room Code
// @Description Bearer callers must supply guildID and receive only roomCode from the verified game snapshot. Basic Auth uses the legacy global lookup and returns connectCode too.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags game
// @Accept json
// @Produce json
// @Param connectCode query string true "Connect Code"
// @Param guildID query string false "Guild ID (required for bearer authentication)"
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

		if c.GetBool(memberRequestKey) {
			// Bind the connect code to the authorized guild before accessing the
			// global room-code key. A code alone is not proof of membership.
			state, err := store.GameState(c.Request.Context(), c.Query("guildID"), connectCode)
			if errors.Is(err, redis.Nil) {
				c.Status(http.StatusNotFound)
				return
			}
			if err != nil {
				c.Status(http.StatusServiceUnavailable)
				return
			}
			if _, err := memberGameState(state, c.Query("guildID"), connectCode); err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			roomCode, err := memberRoomCode(state)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
			// Use the same verified snapshot, avoiding a second global-key lookup.
			c.JSON(http.StatusOK, map[string]string{"roomCode": roomCode})
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
// @Security DiscordBearer
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
// @Security DiscordBearer
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

// NoticeRequest is the body of POST /admin/notice. The notice stays active until DELETE /admin/notice.
type NoticeRequest struct {
	// Severity is warning or critical. Critical ends every running game and blocks new ones.
	Severity string `json:"severity" example:"warning"`
	Message  string `json:"message" example:"Database maintenance in progress; expect some lag."`
}

const maxNoticeMessageLength = 500

func requireConfiguredPassword(config Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.AdminPassword == "" || config.AdminPassword == "automuteus" {
			c.AbortWithStatusJSON(http.StatusForbidden, HttpError{
				StatusCode: http.StatusForbidden,
				Error:      "set API_ADMIN_PASS to a non-default value to manage notices",
			})
		}
	}
}

// @Summary Get the active platform notice
// @Tags admin
// @Produce json
// @Success 200 {object} notice.Notice
// @Failure 404 {object} HttpError
// @Failure 503 {object} HttpError
// @Security BasicAuth
// @Router /admin/notice [get]
func handleGetNotice(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		n, err := store.ActiveNotice(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "notice storage unavailable"})
			return
		}
		if n == nil {
			c.JSON(http.StatusNotFound, HttpError{StatusCode: http.StatusNotFound, Error: "no active notice"})
			return
		}
		c.JSON(http.StatusOK, n)
	}
}

// @Summary Raise a platform notice
// @Description Shows a banner on every game status message until cleared. A critical notice also ends every
// @Description running game (unmuting everyone, recording the matches as aborted) and blocks /new while active.
// @Tags admin
// @Accept json
// @Produce json
// @Param notice body NoticeRequest true "Notice"
// @Success 200 {object} notice.Notice
// @Failure 400 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 503 {object} HttpError
// @Security BasicAuth
// @Router /admin/notice [post]
func handlePostNotice(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		var req NoticeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid notice body"})
			return
		}
		sev := notice.Severity(strings.ToLower(req.Severity))
		msg := strings.TrimSpace(req.Message)
		switch {
		case !sev.Valid():
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "severity must be warning or critical"})
			return
		case msg == "" || len(msg) > maxNoticeMessageLength:
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: fmt.Sprintf("message must be 1-%d characters", maxNoticeMessageLength)})
			return
		}
		n := notice.Notice{Severity: sev, Message: msg}
		if err := store.RaiseNotice(c.Request.Context(), n); err != nil {
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "failed to raise notice"})
			return
		}
		active, err := store.ActiveNotice(c.Request.Context())
		if err != nil || active == nil {
			c.JSON(http.StatusOK, n)
			return
		}
		c.JSON(http.StatusOK, active)
	}
}

// @Summary Clear the active platform notice
// @Tags admin
// @Produce json
// @Success 204
// @Failure 403 {object} HttpError
// @Failure 503 {object} HttpError
// @Security BasicAuth
// @Router /admin/notice [delete]
func handleDeleteNotice(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		if err := store.ClearNotice(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "failed to clear notice"})
			return
		}
		c.Status(http.StatusNoContent)
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
