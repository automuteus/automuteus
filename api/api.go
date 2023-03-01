package api

import (
	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/docs"
	"github.com/automuteus/automuteus/v8/pkg"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/redis"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"strings"
)

type Api struct {
	official          bool
	url               string
	adminPass         string
	discordSession    *discordgo.Session
	redisDriver       redis.Driver
	storageInterface  storage.StorageInterface
	postgresInterface storage.PsqlInterface
}

func NewApi(official bool, url, adminPass string, discordSession *discordgo.Session, driver redis.Driver, storageInterface storage.StorageInterface, psqlInterface storage.PsqlInterface) *Api {
	return &Api{
		official:          official,
		url:               url,
		adminPass:         adminPass,
		discordSession:    discordSession,
		redisDriver:       driver,
		storageInterface:  storageInterface,
		postgresInterface: psqlInterface,
	}
}

func (api *Api) StartServer(port string) error {
	r := gin.Default()

	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Title = "AutoMuteUs"
	docs.SwaggerInfo.Version = pkg.Version
	docs.SwaggerInfo.Description = "AutoMuteUs Bot API"
	var schemes []string
	host := api.url
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
	botGroup.GET("/info", handleGetInfo(api))
	botGroup.GET("/commands", handleGetCommands())

	// TODO in the future, I'd like this to receive a Discord Access Token
	// that way, any user that is logged in via Discord (not only through the web UI)
	// can get info about a game going on in a guild that they're a member of...
	gameGroup := r.Group("/game", gin.BasicAuth(gin.Accounts{
		"admin": api.adminPass,
	}))
	gameGroup.GET("/state", handleGetGameState(api))

	// TODO same as above, but we also need to check the User's permissions within the server in question
	// (aka if user is not a bot admin for a guild, they can't change that guild's settings)
	guildGroup := r.Group("/guild", gin.BasicAuth(gin.Accounts{
		"admin": api.adminPass,
	}))
	guildGroup.GET("/settings", handleGetGuildSettings(api))
	guildGroup.GET("/premium", handleGetGuildPremium(api))
	guildGroup.POST("/premium/transfer", handlePostGuildPremiumTransfer(api))
	guildGroup.POST("/premium/subserver", handlePostGuildPremiumAddGoldSubserver(api))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// TODO add endpoints for notable player information, like total games played, num wins, etc

	// TODO properly configure CORS -_-
	return r.Run(":" + port)
}

// BotInfo godoc
// @Summary Get Bot Info
// @Schemes GET
// @Description Get basic information about the bot
// @Tags bot
// @Accept json
// @Produce json
// @Success 200 {object} discord.BotInfo
// @Router /bot/info [get]
func handleGetInfo(api *Api) func(c *gin.Context) {
	return func(c *gin.Context) {
		info := redis.GetApiInfo(api.redisDriver, api.postgresInterface, api.discordSession)
		c.JSON(http.StatusOK, info)
	}
}

// BotCommands godoc
// @Summary Get Bot Commands
// @Schemes GET
// @Description Get all Discord commands that the bot implements
// @Tags bot
// @Accept json
// @Produce json
// @Success 200 {object} []discordgo.ApplicationCommand
// @Router /bot/commands [get]
func handleGetCommands() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, command.All)
	}
}

// GetGameState godoc
// @Summary Get Game State
// @Schemes GET
// @Description Get the current state of a running game
// @Security BasicAuth
// @Tags game
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param connectCode query string true "Connect Code"
// @Success 200 {object} discord.GameState
// @Failure 400 {string} HttpError
// @Failure 500 {object} nil
// @Router /game/state [get]
func handleGetGameState(api *Api) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		connectCode := c.Query("connectCode")
		if len(connectCode) != 8 {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid connect code",
			})
			return
		}
		gsr := discord.GameStateRequest{
			GuildID:     guildID,
			ConnectCode: connectCode,
		}

		if !api.redisDriver.CheckDiscordGameStateKey(gsr) {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "no game status found with those details",
			})
			return
		}

		state := api.redisDriver.GetReadOnlyDiscordGameState(gsr)
		if state == nil {
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, state)
	}
}

// GetGuildSettings godoc
// @Summary Get Guild Settings
// @Schemes GET
// @Description Get the settings for a given guild
// @Security BasicAuth
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} settings.GuildSettings
// @Failure 400 {string} HttpError
// @Failure 404 {string} HttpError
// @Failure 500 {object} nil
// @Router /guild/settings [get]
func handleGetGuildSettings(api *Api) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		exists := api.storageInterface.GuildSettingsExists(guildID)
		if !exists {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusNotFound,
				Error:      "No settings found for that GuildID",
			})
			return
		}
		settings := api.storageInterface.GetGuildSettings(guildID)
		c.JSON(http.StatusOK, settings)
	}
}

// GetGuildPremium godoc
// @Summary Get Guild Premium
// @Schemes GET
// @Description Get the premium status for a given guild
// @Security BasicAuth
// @Tags guild premium
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} premium.PremiumRecord
// @Failure 400 {string} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/premium [get]
func handleGetGuildPremium(api *Api) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		tier, days, err := api.postgresInterface.GetGuildOrUserPremiumStatus(api.official, nil, guildID, "")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, premium.PremiumRecord{
			Tier: tier,
			Days: days,
		})
	}
}

// TransferPremium godoc
// @Summary Transfer premium
// @Schemes POST
// @Description Transfer premium from one guild to another
// @Security BasicAuth
// @Tags guild premium
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param   GuildIDRequest body GuildIDRequest true "Guild to transfer premium to"
// @Success 202 {object} string
// @Failure 400 {string} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/premium/transfer [post]
func handlePostGuildPremiumTransfer(api *Api) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		var p GuildIDRequest
		if err := c.ShouldBindBodyWith(&p, binding.JSON); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		if discord.ValidateSnowflake(p.GuildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid transfer guild ID",
			})
			return
		}
		if err := api.postgresInterface.TransferPremium(guildID, p.GuildID); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		c.JSON(http.StatusAccepted, "Premium transferred successfully")
	}
}

// AddPremiumSubserver godoc
// @Summary Add a premium subserver
// @Schemes POST
// @Description Add a subserver inheritance to a guild with sufficient premium
// @Security BasicAuth
// @Tags guild premium
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param   GuildIDRequest body GuildIDRequest true "Subserver to inherit premium"
// @Success 202 {object} string
// @Failure 400 {string} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/premium/subserver [post]
func handlePostGuildPremiumAddGoldSubserver(api *Api) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}
		var p GuildIDRequest
		if err := c.ShouldBindBodyWith(&p, binding.JSON); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		if discord.ValidateSnowflake(p.GuildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid subserver guild ID",
			})
			return
		}
		if err := api.postgresInterface.AddGoldSubServer(guildID, p.GuildID); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
		c.JSON(http.StatusAccepted, "Premium subserver status updated")
	}
}

type GuildIDRequest struct {
	GuildID string `json:"guildID"`
}

type HttpError struct {
	StatusCode int
	Error      string
}
