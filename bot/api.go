package bot

import (
	"github.com/automuteus/automuteus/v7/bot/command"
	"github.com/automuteus/automuteus/v7/docs"
	"github.com/automuteus/automuteus/v7/pkg/discord"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"os"
	"strings"
)

func (bot *Bot) StartAPIServer(port string) {
	r := gin.Default()

	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Title = "AutoMuteUs"
	docs.SwaggerInfo.Version = bot.version
	docs.SwaggerInfo.Description = "AutoMuteUs Bot API"
	var schemes []string
	host := os.Getenv("API_SERVER_BASE")
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
	host += ":" + port
	docs.SwaggerInfo.Host = host
	docs.SwaggerInfo.Schemes = schemes

	botGroup := r.Group("/bot")
	botGroup.GET("/info", handleGetInfo(bot))
	botGroup.GET("/commands", handleGetCommands())

	gameGroup := r.Group("/game", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	gameGroup.GET("/state", handleGetGameState(bot))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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

// GetGameState godoc
// @Summary Get Game State
// @Schemes GET
// @Description Get the current state of a running game
// @Security BasicAuth
// @Tags game
// @Accept json
// @Produce json
// @Param guildID query string true "Game Guild ID"
// @Param connectCode query string true "Game Connect Code"
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

type HttpError struct {
	StatusCode int
	Error      string
}
