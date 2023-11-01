package bot

import (
	_ "embed"
	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/docs"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"html/template"
	"net/http"
	"os"
	"strings"
)

//go:embed templates/link.tmpl
var linkTemplateFileContents string

func (bot *Bot) StartAPIServer(port string) {
	r := gin.Default()

	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Title = "AutoMuteUs"
	docs.SwaggerInfo.Version = bot.version
	docs.SwaggerInfo.Description = "AutoMuteUs Bot API"
	var schemes []string
	host := os.Getenv("API_SERVER_URL")
	if host == "" {
		host = "http://localhost"
	}
	adminPassword := os.Getenv("API_ADMIN_PASS")
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
	botGroup.GET("/info", handleGetInfo(bot))
	botGroup.GET("/commands", handleGetCommands())

	// TODO in the future, I'd like this to receive a Discord Access Token
	// that way, any user that is logged in via Discord (not only through the web UI)
	// can get info about a game going on in a guild that they're a member of...
	gameGroup := r.Group("/game", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	gameGroup.GET("/state", handleGetGameState(bot))

	// TODO same as above, but we also need to check the User's permissions within the server in question
	// (aka if user is not a bot admin for a guild, they can't change that guild's settings)
	guildGroup := r.Group("/guild", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	guildGroup.GET("/settings", handleGetGuildSettings(bot))
	guildGroup.GET("/premium", handleGetGuildPremium(bot))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/open/link", handleGetOpenAmongUsCapture(bot))

	// TODO add endpoints for notable player information, like total games played, num wins, etc

	// TODO properly configure CORS -_-
	r.Run(":" + port)
}

// BotInfo godoc
// @Summary Get Bot Info
// @Schemes GET
// @Description Get basic information about the bot
// @Tags bot
// @Accept json
// @Produce json
// @Success 200 {object} command.BotInfo
// @Router /bot/info [get]
func handleGetInfo(bot *Bot) func(c *gin.Context) {
	return func(c *gin.Context) {
		info := bot.getInfo()
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

// Open AmongUsCapture
// @Summary Get AmongUsCapture
// @Schemes GET
// @Description Return html that open AmongUsCapture
// @Produce {string} string "text/html"
// @Success 200 {string} string "text/html"
// @Router /open/link [get]
func handleGetOpenAmongUsCapture(bot *Bot) func(c *gin.Context) {
	return func(c *gin.Context) {
		connectCode := c.Query("connectCode")
		if len(connectCode) != 8 {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid connect code",
			})
			return
		}
		hyperlink, _, _ := formCaptureURL(bot.url, connectCode)
		t, err := template.New("template").Parse(linkTemplateFileContents)
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpError{
				StatusCode: http.StatusInternalServerError,
				Error:      err.Error(),
			})
			return
		}
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
// @Schemes GET
// @Description Get the current state of a running game
// @Security BasicAuth
// @Tags game
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param connectCode query string true "Connect Code"
// @Success 200 {object} GameState
// @Failure 400 {string} HttpError
// @Failure 500 {object} nil
// @Router /game/state [get]
func handleGetGameState(bot *Bot) func(c *gin.Context) {
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
		gsr := GameStateRequest{
			GuildID:     guildID,
			ConnectCode: connectCode,
		}
		key := bot.RedisInterface.getDiscordGameStateKey(gsr)
		if key == "" {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "no game status found with those details",
			})
			return
		}

		state := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
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
func handleGetGuildSettings(bot *Bot) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		exists := bot.StorageInterface.GuildSettingsExists(guildID)
		if !exists {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusNotFound,
				Error:      "No settings found for that GuildID",
			})
			return
		}
		settings := bot.StorageInterface.GetGuildSettings(guildID)
		c.JSON(http.StatusOK, settings)
	}
}

// GetGuildPremium godoc
// @Summary Get Guild Premium
// @Schemes GET
// @Description Get the premium status for a given guild
// @Security BasicAuth
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} premium.PremiumRecord
// @Failure 400 {string} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/premium [get]
func handleGetGuildPremium(bot *Bot) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		tier, days, err := bot.PostgresInterface.GetGuildOrUserPremiumStatus(bot.official, nil, guildID, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpError{
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

type HttpError struct {
	StatusCode int
	Error      string
}
