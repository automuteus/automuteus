package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

// StatsPlayer is how a user is shown on the stats page: the name to call them and a picture. Nickname and
// guild avatar are specific to the guild; the rest is the user's global profile.
type StatsPlayer struct {
	Username string `json:"username"`
	// GlobalName is the display name the user chose for all of Discord, if any.
	GlobalName string `json:"globalName,omitempty"`
	// Nickname is the user's name in this guild, if they set one.
	Nickname string `json:"nickname,omitempty"`
	// Avatar is a Discord CDN image URL: the user's avatar for this guild, else their global avatar, else the
	// default Discord assigns them. Always set when the profile was resolved through Discord.
	Avatar string `json:"avatar,omitempty"`
}

// ProfileFetcher looks a user up with the bot's own credentials. errChannelNotFound means Discord does not know
// the user (or the bot may not see them); errChannelUnavailable is a rate limit, outage, or bad token.
type ProfileFetcher interface {
	FetchProfile(ctx context.Context, guildID, userID string) (StatsPlayer, error)
}

const (
	// profileCacheTTL matches the bot's own name cache. Names and avatars change rarely, and one build of a
	// guild's stats names at most a few dozen users.
	profileCacheTTL = 12 * time.Hour
	// profileMissTTL is how long a user Discord did not know is left alone before asking again.
	profileMissTTL = time.Hour
	// profileFetchWorkers bounds concurrent Discord lookups so a first view of a busy guild does not burst the
	// member route's rate limit; the verifier's cooldowns catch anything that gets through.
	profileFetchWorkers = 4
)

// profileFetchBudget bounds the whole Discord phase of one rollup. Profiles are optional, and the stats request
// has its own deadline, so a slow or unreachable Discord must not hold the response: whoever is unresolved when
// the budget runs out gets the bot's cached name instead. A variable so tests can shorten it.
var profileFetchBudget = 4 * time.Second

// FetchProfile reads the user's member record in the guild, for their nickname and guild avatar, and falls back
// to their global user record when they are no longer a member so old leaderboard entries keep a name.
func (v *discordChannelVerifier) FetchProfile(ctx context.Context, guildID, userID string) (StatsPlayer, error) {
	if discord.ValidateSnowflake(guildID) != nil || discord.ValidateSnowflake(userID) != nil {
		return StatsPlayer{}, errChannelNotFound
	}
	var member discordMember
	err := v.get(ctx, "/guilds/"+guildID+"/members/"+userID, &member)
	if err == nil {
		if member.User == nil || member.User.ID != userID {
			return StatsPlayer{}, errChannelUnavailable
		}
		p := StatsPlayer{Username: member.User.Username, GlobalName: member.User.GlobalName, Nickname: member.Nick}
		if member.Avatar != "" && avatarHash.MatchString(member.Avatar) {
			p.Avatar = "https://cdn.discordapp.com/guilds/" + guildID + "/users/" + userID + "/avatars/" + member.Avatar + ".png?size=128"
		} else {
			p.Avatar = userAvatarURL(member.User)
		}
		return p, nil
	}
	if !errors.Is(err, errChannelNotFound) {
		return StatsPlayer{}, err
	}
	var user discordUser
	if err := v.get(ctx, "/users/"+userID, &user); err != nil {
		return StatsPlayer{}, err
	}
	if user.ID != userID {
		return StatsPlayer{}, errChannelUnavailable
	}
	return StatsPlayer{Username: user.Username, GlobalName: user.GlobalName, Avatar: userAvatarURL(&user)}, nil
}

// The pinned discordgo predates global names, so the two Discord objects are decoded into local types.
type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
}

type discordMember struct {
	User   *discordUser `json:"user"`
	Nick   string       `json:"nick"`
	Avatar string       `json:"avatar"`
}

// avatarHash is what Discord issues: hex, with an a_ prefix for animated ones. Anything else is not put in a URL.
var avatarHash = regexp.MustCompile(`^(a_)?[0-9a-f]{32}$`)

// userAvatarURL is the user's global avatar, or the default Discord shows for them: users on the new username
// system get one of six by ID, older ones with a discriminator one of five by discriminator.
func userAvatarURL(user *discordUser) string {
	if user.Avatar != "" && avatarHash.MatchString(user.Avatar) {
		return "https://cdn.discordapp.com/avatars/" + user.ID + "/" + user.Avatar + ".png?size=128"
	}
	return DefaultAvatarURL(user.ID, user.Discriminator)
}

// DefaultAvatarURL is the avatar Discord assigns a user who never set one.
func DefaultAvatarURL(userID, discriminator string) string {
	index := uint64(0)
	if d, err := strconv.ParseUint(discriminator, 10, 64); err == nil && d != 0 {
		index = d % 5
	} else if id, err := strconv.ParseUint(userID, 10, 64); err == nil {
		index = (id >> 22) % 6
	}
	return "https://cdn.discordapp.com/embed/avatars/" + strconv.FormatUint(index, 10) + ".png"
}

// resolvePlayers finds a name and picture for each user named on the boards, in this order: the API's own
// profile cache; Discord, through the bot's credentials, when a fetcher is configured (the answer is cached,
// including a miss); and finally the names the bot cached for the guild, which carry no picture. Users nothing
// knows are absent from the result and the page shows their ID. Lookups are a convenience, so no failure here
// fails the rollup.
func resolvePlayers(ctx context.Context, client *redis.Client, fetcher ProfileFetcher, guildID string, userIDs []string) map[string]StatsPlayer {
	players := make(map[string]StatsPlayer, len(userIDs))
	if len(userIDs) == 0 {
		return players
	}
	// pending is who Discord may be asked about; unresolved is who is left for the bot's name cache.
	pending, unresolved := userIDs, []string{}
	if client != nil {
		cached, err := cachedProfiles(ctx, client, guildID, userIDs)
		if err != nil {
			log.Printf("Guild %s stats: could not read cached profiles: %v\n", guildID, err)
		} else {
			pending = pending[:0:0]
			for _, id := range userIDs {
				p, known := cached[id]
				switch {
				case !known:
					pending = append(pending, id)
				case p.Username != "":
					players[id] = p
				default:
					// A cached miss is not asked about again until it expires, but the bot may still know a name.
					unresolved = append(unresolved, id)
				}
			}
		}
	}
	if fetcher != nil && len(pending) > 0 {
		unresolved = append(unresolved, fetchProfiles(ctx, client, fetcher, guildID, pending, players)...)
	} else {
		unresolved = append(unresolved, pending...)
	}
	if client != nil && len(unresolved) > 0 {
		names, err := cachedPlayerNames(ctx, client, guildID, unresolved)
		if err != nil {
			log.Printf("Guild %s stats: could not read cached player names: %v\n", guildID, err)
			return players
		}
		for id, p := range names {
			players[id] = p
		}
	}
	return players
}

// fetchProfiles asks Discord about each pending user and records the answers in players and the cache. It
// returns the users it could not settle, so the caller can try the bot's name cache for them.
func fetchProfiles(ctx context.Context, client *redis.Client, fetcher ProfileFetcher, guildID string, pending []string, players map[string]StatsPlayer) []string {
	type outcome struct {
		id      string
		profile StatsPlayer
		err     error
	}
	results := make([]outcome, len(pending))
	// The group's context is cancelled once Wait returns, so the cache writes below use the caller's.
	budget, cancel := context.WithTimeout(ctx, profileFetchBudget)
	defer cancel()
	g, gctx := errgroup.WithContext(budget)
	g.SetLimit(profileFetchWorkers)
	for i, id := range pending {
		g.Go(func() error {
			p, err := fetcher.FetchProfile(gctx, guildID, id)
			results[i] = outcome{id, p, err}
			return nil
		})
	}
	_ = g.Wait()
	unresolved := make([]string, 0)
	for _, r := range results {
		switch {
		case r.err == nil:
			players[r.id] = r.profile
			cacheProfile(ctx, client, guildID, r.id, r.profile, profileCacheTTL)
		case errors.Is(r.err, errChannelNotFound):
			cacheProfile(ctx, client, guildID, r.id, StatsPlayer{}, profileMissTTL)
			unresolved = append(unresolved, r.id)
		default:
			// Unavailable, or the budget ran out: nothing is cached, so the next build tries again.
			unresolved = append(unresolved, r.id)
		}
	}
	return unresolved
}

func cachedProfiles(ctx context.Context, client *redis.Client, guildID string, userIDs []string) (map[string]StatsPlayer, error) {
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = rediskey.CachedPlayerProfile(id, guildID)
	}
	values, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]StatsPlayer, len(userIDs))
	for i, v := range values {
		s, ok := v.(string)
		if !ok {
			continue
		}
		var p StatsPlayer
		if json.Unmarshal([]byte(s), &p) != nil {
			continue // an unreadable record is treated as absent and refetched
		}
		out[userIDs[i]] = p
	}
	return out, nil
}

func cacheProfile(ctx context.Context, client *redis.Client, guildID, userID string, p StatsPlayer, ttl time.Duration) {
	if client == nil {
		return
	}
	blob, err := json.Marshal(p)
	if err != nil {
		return
	}
	if err := client.Set(ctx, rediskey.CachedPlayerProfile(userID, guildID), blob, ttl).Err(); err != nil {
		log.Printf("Guild %s stats: could not cache profile for %s: %v\n", guildID, userID, err)
	}
}
