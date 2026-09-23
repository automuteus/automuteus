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
	"github.com/automuteus/automuteus/v8/pkg/locale"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
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
	// Settings returns a guild's settings and the version of its row, for ETags and conditional writes.
	Settings(context.Context, string) (*settings.GuildSettings, storage.SettingsVersion, error)
	// SetSettings writes a complete, validated document only if the row is still at the given version, and
	// returns storage.ErrSettingsConflict otherwise.
	SetSettings(context.Context, string, *settings.GuildSettings, storage.SettingsVersion) error
	// ReserveSettingsWrite consumes one slot of the per-guild settings write budget. A zero duration means the
	// write may proceed; a positive one is how long the caller should wait. Errors mean the budget could not
	// be checked and the write must not proceed.
	ReserveSettingsWrite(context.Context, string) (time.Duration, error)
	Premium(context.Context, string) (premium.PremiumRecord, error)
	// BotInGuild reports whether the bot currently has a member record for the guild, from the set the bot
	// maintains on GuildCreate and GuildDelete. It reflects the last gateway events the bot saw, not a live
	// Discord lookup, so a removal that happened while every shard was offline is not visible until the bot
	// next sees the guild.
	BotInGuild(context.Context, string) (bool, error)
	Ping(context.Context) error
	ActiveNotice(context.Context) (*notice.Notice, error)
	RaiseNotice(context.Context, notice.Notice) error
	ClearNotice(context.Context) error
}

type Config struct {
	// GuildVerifier is injectable for tests; nil uses Discord HTTPS endpoints.
	GuildVerifier GuildVerifier
	// AccessCacheTTL is how long a verified read authorization for one (token, guild) is reused before Discord is
	// asked again. Zero means DefaultAccessCacheTTL; negative disables caching. Writes always verify live.
	AccessCacheTTL time.Duration
	// ListCacheTTL is how long GET /guild/roles and GET /guild/channels reuse a guild's lists before asking Discord
	// again. Zero means DefaultListCacheTTL; negative disables caching. PATCH validates roles against a live list.
	ListCacheTTL  time.Duration
	Version       string
	Commit        string
	ServerURL     string
	AdminPassword string
	CaptureHost   string
	Official      bool
	// BotToken lets the API verify, with the bot's own credentials, that a channel a client names belongs to the
	// guild being edited. Without it (and without an injected ChannelVerifier) the summary channel cannot be
	// changed through the API.
	BotToken string
	// ChannelVerifier is injectable for tests; nil uses Discord HTTPS endpoints when BotToken is set.
	ChannelVerifier ChannelVerifier
	// RoleLister is injectable for tests; nil uses the Discord-backed channel verifier when BotToken is set, since
	// the same bot credentials list roles. Without one, operator role IDs are accepted as typed.
	RoleLister RoleLister
	// ChannelLister is injectable for tests; nil uses the Discord-backed channel verifier when BotToken is set.
	// Without one, GET /guild/channels answers 501 and clients fall back to typing a channel ID.
	ChannelLister ChannelLister
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
	botGroup.GET("/settings/defaults", handleGetSettingsDefaults())

	verifier := config.GuildVerifier
	if verifier == nil {
		verifier = newDiscordVerifier()
	}
	ttl := config.AccessCacheTTL
	if ttl == 0 {
		ttl = DefaultAccessCacheTTL
	}
	access := newAccessCache(ttl, nil)
	gameGroup := r.Group("/game", guildAuthentication(config, verifier, access, ReadGame))
	gameGroup.GET("/state", handleGetGameState(store))
	gameGroup.GET("/roomcode", handleGetRoomCode(store))
	guildGroup := r.Group("/guild")
	guildGroup.GET("/settings", guildAuthentication(config, verifier, access, ReadSettings), handleGetGuildSettings(store))
	channels := config.ChannelVerifier
	if channels == nil && config.BotToken != "" {
		channels = newDiscordChannelVerifier(config.BotToken)
	}
	roles := config.RoleLister
	if roles == nil {
		if lister, ok := channels.(RoleLister); ok {
			roles = lister
		}
	}
	channelList := config.ChannelLister
	if channelList == nil {
		if lister, ok := channels.(ChannelLister); ok {
			channelList = lister
		}
	}
	guildGroup.PATCH("/settings", guildAuthentication(config, verifier, access, WriteSettings), handleUpdateGuildSettings(store, channels, roles))
	guildGroup.GET("/premium", guildAuthentication(config, verifier, access, ReadPremium), handleGetGuildPremium(store))
	guildGroup.GET("/bot", guildAuthentication(config, verifier, access, ReadBotPresence), handleGetGuildBot(store))
	guildGroup.GET("/channel", guildAuthentication(config, verifier, access, ReadSettings), handleGetGuildChannel(channels))
	// The list routes are served from a short per-guild cache so a page held on refresh, or a busy guild, costs
	// Discord a few calls a minute rather than a few per load. PATCH keeps the live lister for role validation.
	listTTL := config.ListCacheTTL
	if listTTL == 0 {
		listTTL = DefaultListCacheTTL
	}
	var listedRoles RoleLister
	if roles != nil {
		listedRoles = cachedRoleLister{newListCache(listTTL, nil, roles.ListRoles)}
	}
	var listedChannels ChannelLister
	if channelList != nil {
		listedChannels = cachedChannelLister{newListCache(listTTL, nil, channelList.ListChannels)}
	}
	guildGroup.GET("/roles", guildAuthentication(config, verifier, access, ReadSettings), handleGetGuildRoles(listedRoles))
	// Channel names can be private, so listing them takes the write permission rather than membership.
	guildGroup.GET("/channels", guildAuthentication(config, verifier, access, WriteSettings), handleGetGuildChannels(listedChannels))

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

// SettingsDefaults godoc
// @Summary Get Default Guild Settings
// @Description The settings every guild starts with, and what GET /guild/settings returns for a guild that never
// @Description changed anything. Clients compare against this to show which settings a guild has customised and to
// @Description offer a reset. Not guild-specific and not secret, so no authentication is required.
// @Tags bot
// @Accept json
// @Produce json
// @Success 200 {object} settings.GuildSettings
// @Router /bot/settings/defaults [get]
func handleGetSettingsDefaults() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, settings.MakeGuildSettings())
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
// @Header 200 {string} ETag "Version of the stored settings, for If-Match on PATCH"
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

		settings, version, err := store.Settings(c.Request.Context(), guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{
				StatusCode: http.StatusServiceUnavailable,
				Error:      "Unable to load guild settings",
			})
			return
		}
		c.Header("ETag", settingsETag(version))
		c.JSON(http.StatusOK, settings)
	}
}

// Settings writes: one guild may be rewritten at most SettingsWriteLimit times per SettingsWriteWindow, and a
// request body may not exceed maxSettingsBody bytes (a complete document with full ID lists is a few KiB).
const (
	SettingsWriteLimit  = 10
	SettingsWriteWindow = time.Minute
	maxSettingsBody     = 64 << 10
)

// SettingsValidationError is the 400 response for a settings document that decoded but failed validation. Every
// offending field is listed so a client can fix them all in one round trip.
type SettingsValidationError struct {
	StatusCode int                   `json:"StatusCode"`
	Error      string                `json:"Error"`
	Fields     []settings.FieldError `json:"fields"`
}

// UpdateGuildSettings godoc
// @Summary Update Guild Settings
// @Description Change settings for a guild. The body is a JSON object with any subset of the fields returned by GET
// @Description /guild/settings; fields that are present replace the stored value and fields that are absent keep it.
// @Description Voice rule and delay rows are replaced whole, so a row must list every entry. Unknown fields, null
// @Description values, and values outside the ranges the /settings slash command accepts are rejected without saving.
// @Description Requires the guild owner or the Discord Administrator or Manage Server permission. Changing a
// @Description premium-only setting
// @Description (match summary options, auto refresh, leaderboard options, spectator muting, room code display) on a
// @Description guild without premium is refused with 403 listing those fields. A new matchSummaryChannelID must pass
// @Description the same checks as GET /guild/channel: visible to the bot, in this guild, text-capable, and granting
// @Description the bot View Channel, Send Messages, and Embed Links. A changed permissionRoleIDs list must name roles of
// @Description this guild (see GET /guild/roles) when the API has bot credentials.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param If-Match header string false "ETag from a previous GET or PATCH; the write is refused with 412 if the settings changed since"
// @Param settings body settings.GuildSettings true "Fields to change"
// @Success 200 {object} settings.GuildSettings "The stored settings after the change"
// @Header 200 {string} ETag "Version of the stored settings after the change"
// @Failure 400 {object} SettingsValidationError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 409 {object} HttpError "Settings were changed concurrently; reload and retry"
// @Failure 412 {object} HttpError "If-Match did not match the stored version"
// @Failure 413 {object} HttpError
// @Failure 429 {object} HttpError
// @Failure 501 {object} HttpError "Summary channel changes need DISCORD_BOT_TOKEN on the API"
// @Failure 503 {object} HttpError
// @Router /guild/settings [patch]
func handleUpdateGuildSettings(store Store, channels ChannelVerifier, roles RoleLister) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}

		// Every attempt counts, valid or not: the budget bounds how often a guild's row is touched and how often
		// Discord is asked to revalidate a writer.
		retryAfter, err := store.ReserveSettingsWrite(ctx, guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to save guild settings"})
			return
		}
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			c.JSON(http.StatusTooManyRequests, HttpError{StatusCode: http.StatusTooManyRequests,
				Error: fmt.Sprintf("Too many settings changes for this guild; at most %d per %s", SettingsWriteLimit, SettingsWriteWindow)})
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBody))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				c.JSON(http.StatusRequestEntityTooLarge, HttpError{StatusCode: http.StatusRequestEntityTooLarge,
					Error: fmt.Sprintf("settings document exceeds %d bytes", maxSettingsBody)})
				return
			}
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "unable to read settings document"})
			return
		}

		// Decode over the stored settings so absent fields keep their values. Gaps in a legacy row are filled with
		// the defaults first, the same substitutions the getters make, so an untouched legacy zero never fails
		// validation of a field the client did not send.
		current, version, err := store.Settings(ctx, guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to load guild settings"})
			return
		}
		// A client that edits what it loaded sends the ETag back; if the settings moved on since, it is told to
		// reload rather than have its stale view applied over someone else's change.
		if !ifMatchAllows(c.GetHeader("If-Match"), version) {
			c.Header("ETag", settingsETag(version))
			c.JSON(http.StatusPreconditionFailed, HttpError{StatusCode: http.StatusPreconditionFailed,
				Error: "settings changed since they were loaded; reload and retry"})
			return
		}
		current.FillDefaults()
		premiumBefore := current.PremiumSnapshot()
		channelBefore := current.MatchSummaryChannelID
		rolesBefore := append([]string(nil), current.PermissionRoleIDs...)
		if err := settings.UnmarshalStrict(body, current); err != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid settings document: " + err.Error()})
			return
		}
		if err := current.Validate(locale.GetLanguages()); err != nil {
			respondSettingsValidation(c, err)
			return
		}

		// The same settings the /settings slash command reserves for premium guilds are reserved here. Only a change
		// counts: echoing a stored value back is allowed, so a client may always resubmit the document it was given.
		if changed := premiumBefore.Changed(current); len(changed) > 0 {
			record, err := store.Premium(ctx, guildID)
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to check premium status"})
				return
			}
			if premium.IsExpired(record.Tier, record.Days) {
				fields := make([]settings.FieldError, len(changed))
				for i, name := range changed {
					fields[i] = settings.FieldError{Field: name, Message: "changing this setting requires premium"}
				}
				c.JSON(http.StatusForbidden, SettingsValidationError{
					StatusCode: http.StatusForbidden,
					Error:      "premium required to change: " + strings.Join(changed, ", "),
					Fields:     fields,
				})
				return
			}
		}

		// Operator role IDs decide who may control games, so a typo would silently lock people out. When the API
		// has bot credentials, a changed list is checked against the guild's real roles; without them the IDs are
		// accepted as typed, as they always were. The same list backs GET /guild/roles so a client can pick first.
		if roles != nil && !sameStrings(rolesBefore, current.PermissionRoleIDs) {
			unknown, err := unknownOperatorRoles(ctx, roles, guildID, current.PermissionRoleIDs)
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to verify operator roles"})
				return
			}
			if len(unknown) > 0 {
				respondSettingsFields(c, http.StatusBadRequest, "unknown operator roles", unknown...)
				return
			}
		}

		// The summary channel is where the bot will post on the caller's behalf. Validation only proved it is a
		// snowflake; it must also be a text channel of this guild that the bot can post embeds into, or an
		// administrator of one guild could aim the bot at a channel in another guild it shares, or at one where
		// summaries would silently never appear. Clearing the channel needs no lookup. The same check backs
		// GET /guild/channel so a client can ask first.
		if current.MatchSummaryChannelID != "" && current.MatchSummaryChannelID != channelBefore {
			if channels == nil {
				c.JSON(http.StatusNotImplemented, HttpError{StatusCode: http.StatusNotImplemented,
					Error: "changing matchSummaryChannelID requires the API to be configured with DISCORD_BOT_TOKEN"})
				return
			}
			check, err := checkSummaryChannel(ctx, channels, guildID, current.MatchSummaryChannelID)
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to verify summary channel"})
				return
			}
			if !check.OK {
				respondSettingsFields(c, http.StatusBadRequest, "invalid summary channel",
					settings.FieldError{Field: "matchSummaryChannelID", Message: strings.Join(check.Problems, "; ")})
				return
			}
		}

		// Conditional on the version read above: a concurrent write in between is a conflict, never a silent
		// overwrite of the other writer's fields.
		if err := store.SetSettings(ctx, guildID, current, version); err != nil {
			var verrs settings.ValidationErrors
			switch {
			case errors.Is(err, storage.ErrSettingsConflict):
				c.JSON(http.StatusConflict, HttpError{StatusCode: http.StatusConflict,
					Error: "settings were changed concurrently; reload and retry"})
			case errors.As(err, &verrs):
				respondSettingsValidation(c, err)
			default:
				log.Println(err)
				c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to save guild settings"})
			}
			return
		}
		log.Printf("[API] Guild %s settings updated by %s (version %d -> %d): %s", guildID, requestActor(c), version, version+1, sentFields(body))
		c.Header("ETag", settingsETag(version+1))
		c.JSON(http.StatusOK, current)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func respondSettingsValidation(c *gin.Context, err error) {
	var verrs settings.ValidationErrors
	errors.As(err, &verrs)
	respondSettingsFields(c, http.StatusBadRequest, fmt.Sprintf("%d invalid setting(s)", len(verrs)), verrs...)
}

func respondSettingsFields(c *gin.Context, status int, message string, fields ...settings.FieldError) {
	c.JSON(status, SettingsValidationError{StatusCode: status, Error: message, Fields: fields})
}

// settingsETag renders a settings row version as a strong ETag.
func settingsETag(version storage.SettingsVersion) string {
	return fmt.Sprintf(`"%d"`, version)
}

// ifMatchAllows reports whether an If-Match header permits a write against the current version. No header means
// the client did not ask for the check. "*" matches any state. Otherwise any listed tag equal to the current
// version's ETag matches; weak indicators and quotes are tolerated.
func ifMatchAllows(header string, version storage.SettingsVersion) bool {
	header = strings.TrimSpace(header)
	if header == "" || header == "*" {
		return true
	}
	want := strconv.FormatInt(int64(version), 10)
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		tag = strings.TrimPrefix(tag, "W/")
		tag = strings.Trim(tag, `"`)
		if tag == want {
			return true
		}
	}
	return false
}

// requestActor names who authenticated the request, for audit logs.
func requestActor(c *gin.Context) string {
	if user, ok := c.Get(verifiedUserKey); ok {
		return fmt.Sprintf("Discord user %v", user)
	}
	return "platform admin (Basic Auth)"
}

// sentFields lists the top-level keys a request body changed, sorted, for audit logs. The body has already been
// decoded strictly by the time this runs, so a failure here only affects the log line.
func sentFields(body []byte) string {
	var doc map[string]json.RawMessage
	if json.Unmarshal(body, &doc) != nil {
		return "(unparseable)"
	}
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// BotPresence says whether the bot is a member of a guild, so a client can offer an invite instead of settings
// the bot could never apply.
type BotPresence struct {
	Present bool `json:"present"`
}

// GetGuildBot godoc
// @Summary Get Guild Bot Presence
// @Description Report whether the bot is currently a member of the given guild, based on the guild join and leave
// @Description events the bot has processed. Any member of the guild may ask.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} BotPresence
// @Failure 400 {object} HttpError
// @Failure 503 {object} HttpError
// @Router /guild/bot [get]
func handleGetGuildBot(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{
				StatusCode: http.StatusBadRequest,
				Error:      "invalid guild ID",
			})
			return
		}

		present, err := store.BotInGuild(c.Request.Context(), guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{
				StatusCode: http.StatusServiceUnavailable,
				Error:      "Unable to check bot membership",
			})
			return
		}
		c.JSON(http.StatusOK, BotPresence{Present: present})
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
