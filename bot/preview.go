package bot

import (
	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

// PreviewEmbed is one of the bot's game status embeds rendered from sample data, for eyeballing in a real channel
// without running the bot (see cmd/embedpreview).
type PreviewEmbed struct {
	Name  string
	Embed *discordgo.MessageEmbed
}

// PreviewEmbeds renders every game status embed the bot can send, using the same code paths as live games, and
// applies n (if any) the way an active platform notice would be.
func PreviewEmbeds(sett *settings.GuildSettings, n *notice.Notice) []PreviewEmbed {
	emojis := GlobalAlivenessEmojis

	unlinked := previewState(game.MENU)
	unlinked.Linked = false

	paused := previewState(game.LOBBY)
	paused.Running = false

	previews := []PreviewEmbed{
		{"menu-unlinked", menuMessage(unlinked, emojis, sett)},
		{"lobby", lobbyMessage(previewState(game.LOBBY), emojis, sett)},
		{"lobby-paused", lobbyMessage(paused, emojis, sett)},
		{"tasks", gamePlayMessage(previewState(game.TASKS), emojis, sett)},
		{"discussion", gamePlayMessage(previewState(game.DISCUSS), emojis, sett)},
		{"gameover", gameOverMessage(previewState(game.GAMEOVER), emojis, sett, "<@100000000000000001>, <@100000000000000002> won as Crewmate")},
	}
	for _, p := range previews {
		applyNotice(p.Embed, n, sett)
	}
	return previews
}

// previewState is a plausible mid-game state: four players, three linked to Discord users, one dead.
func previewState(phase game.Phase) *GameState {
	dgs := NewDiscordGameState("100000000000000000")
	dgs.ConnectCode = "PREVIEW1"
	dgs.Linked = true
	dgs.Running = true
	dgs.VoiceChannel = "100000000000000010"
	dgs.GameStateMsg.LeaderID = "100000000000000001"
	dgs.GameData.UpdatePhase(phase)
	dgs.GameData.SetRoomRegionMap("ABCDEF", game.NA.ToString(), game.SKELD)

	players := []struct {
		name   string
		color  int
		alive  bool
		userID string
	}{
		{"Alice", 0, true, "100000000000000001"},
		{"Bob", 1, true, "100000000000000002"},
		{"Carol", 2, phase != game.TASKS && phase != game.DISCUSS, "100000000000000003"},
		{"Dave", 3, true, ""},
	}
	for _, p := range players {
		dgs.GameData.PlayerData[p.name] = amongus.PlayerData{Color: p.color, Name: p.name, IsAlive: p.alive}
		if p.userID != "" {
			dgs.UserData[p.userID] = UserData{
				User:       User{UserID: p.userID, UserName: p.name},
				InGameName: p.name,
			}
		}
	}
	return dgs
}
