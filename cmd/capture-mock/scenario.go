package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

// A scenario plays a scripted sequence of capture events and pauses at
// checkpoints so the operator can confirm, by looking at Discord, that the bot
// reacted as expected. Expectations are derived from bot/eventHandler.go, the
// default voice rules in pkg/game/voice_rule.go and the default phase delays in
// pkg/game/delay.go; update them when that behavior changes.
type scenario struct {
	name        string
	description string
	steps       []step
}

// step is exactly one of: an event to send (send set), a checkpoint to confirm
// (expect set), or a note telling the operator to do something (label only).
// Text may contain {you}, which is replaced by the operator's player name.
type step struct {
	label  string
	send   func(s *session, you string) error
	expect string
}

type checkResult struct {
	expect   string
	observed bool
}

// errAborted is returned when the operator quits a scenario at a checkpoint.
var errAborted = errors.New("scenario aborted")

// self stands for the operator's player wherever a step names a player.
const self = "{you}"

// Other players are fixed so the operator can recognize them in Discord.
const (
	mockBlue   = "Mock Blue"
	mockGreen  = "Mock Green"
	mockPurple = "Mock Purple"
)

func lobby(code string, region game.Region, playMap game.PlayMap) step {
	return step{
		label: fmt.Sprintf("lobby %s, region %s, map %s", code, region.ToString(), game.MapNames[playMap]),
		send: func(s *session, _ string) error {
			return s.lobby(game.Lobby{LobbyCode: code, Region: region, PlayMap: playMap})
		},
	}
}

func phase(p game.Phase) step {
	return step{
		label: fmt.Sprintf("state %s", p.ToString()),
		send:  func(s *session, _ string) error { return s.phase(p) },
	}
}

var actionVerbs = map[game.PlayerAction]string{
	game.JOINED: "joins", game.LEFT: "leaves", game.DIED: "dies", game.CHANGECOLOR: "changes color",
	game.FORCEUPDATED: "is force updated", game.DISCONNECTED: "disconnects", game.EXILED: "is exiled",
}

func playerLabel(name string, action game.PlayerAction, color int) string {
	return fmt.Sprintf("player %s %s (%s)", name, actionVerbs[action], game.GetColorStringForInt(color))
}

func player(name string, action game.PlayerAction, color int, impostor bool) step {
	p := game.Player{
		Action:       action,
		Color:        color,
		IsDead:       action == game.DIED || action == game.EXILED,
		Disconnected: action == game.DISCONNECTED,
	}
	return step{
		label: playerLabel(name, action, color),
		send: func(s *session, you string) error {
			p.Name = strings.ReplaceAll(name, self, you)
			return s.player(p, impostor)
		},
	}
}

func gameover(result game.GameResult) step {
	return step{
		label: fmt.Sprintf("gameover %s", resultNames[result]),
		send:  func(s *session, _ string) error { return s.gameover(result) },
	}
}

func expect(text string) step { return step{expect: text} }
func note(text string) step   { return step{label: text} }

// openLobby is the shared opening: a lobby of four, with the operator linked to
// the red player so that voice changes can be observed on their own account.
func openLobby() []step {
	return []step{
		lobby("TESTCODE", game.NA, game.SKELD),
		player(self, game.JOINED, game.Red, false),
		player(mockBlue, game.JOINED, game.Blue, false),
		player(mockGreen, game.JOINED, game.Green, false),
		player(mockPurple, game.JOINED, game.Purple, false),
		expect("The status message shows lobby code TESTCODE, region North America, map The Skeld, and the four players {you} (red), Mock Blue, Mock Green and Mock Purple."),
		note("Sit in the tracked voice channel and link yourself to {you} with the status message's color dropdown (red), unless you were already auto-linked because {you} is your Discord name."),
		expect("The status message lists your Discord user next to {you} and the linked count reads 1/4."),
	}
}

// startTasks starts a round from the lobby and confirms the alive mute.
func startTasks() []step {
	return []step{
		phase(game.TASKS),
		expect("After about 7 seconds (the lobby-to-tasks delay) you are muted and deafened in voice, and the status message reads Tasks."),
	}
}

var scenarios = []scenario{
	{
		name:        "Full round",
		description: "Lobby, tasks, a meeting, more tasks, and a crewmate victory with nobody dying.",
		steps: concat(openLobby(), startTasks(), []step{
			phase(game.DISCUSS),
			expect("You are unmuted and undeafened immediately, and the status message reads Discussion with everyone alive."),
			phase(game.TASKS),
			expect("After about 7 seconds you are muted and deafened again."),
			gameover(game.HumansByTask),
			expect("A match summary is posted saying crewmates won by tasks and mentioning you as winning as Crewmate, you are unmuted and undeafened within about a second, and the status message returns to the Lobby view."),
		}),
	},
	{
		name:        "Killed during tasks",
		description: "You are killed mid-round; dead players stay silent in meetings and talk freely during tasks.",
		steps: concat(openLobby(), startTasks(), []step{
			player(self, game.DIED, game.Red, false),
			expect("Nothing changes: you stay muted and deafened, and the status message is NOT edited (a dead marker during tasks would leak information). With the unmute-dead-during-tasks setting enabled you would be unmuted instead."),
			phase(game.DISCUSS),
			expect("You are muted but NOT deafened, and the status message now shows {you} as dead."),
			phase(game.TASKS),
			expect("After about 7 seconds you are unmuted and undeafened, since dead players may talk during tasks."),
			gameover(game.ImpostorByKill),
			expect("A match summary is posted saying impostors won by kills, without mentioning you, and the status message returns to the Lobby view with everyone alive again."),
		}),
	},
	{
		name:        "Exiled at a meeting",
		description: "You are voted out; the status message updates at once but your voice state waits for the next phase.",
		steps: concat(openLobby(), startTasks(), []step{
			phase(game.DISCUSS),
			expect("You are unmuted and undeafened."),
			player(self, game.EXILED, game.Red, false),
			expect("The status message updates to show {you} as dead, and your voice state does not change yet (still unmuted and undeafened)."),
			phase(game.TASKS),
			expect("After about 7 seconds you remain unmuted and undeafened, since you are dead during tasks."),
			gameover(game.HumansByVote),
			expect("A match summary is posted saying crewmates won by vote and mentioning you as winning as Crewmate, and the status message returns to the Lobby view."),
		}),
	},
	{
		name:        "Leaving mid-round",
		description: "A player leaves during tasks, and another disconnects at the meeting.",
		steps: concat(openLobby(), startTasks(), []step{
			player(self, game.LEFT, game.Red, false),
			expect("You are unmuted and undeafened, and the status message is NOT edited during tasks."),
			phase(game.DISCUSS),
			expect("The status message reads Discussion and no longer lists {you}; your voice state is unchanged."),
			player(mockBlue, game.DISCONNECTED, game.Blue, false),
			expect("The status message no longer lists Mock Blue."),
			gameover(game.ImpostorDisconnect),
			expect("A match summary is posted saying impostors won by disconnect, and the status message returns to the Lobby view."),
		}),
	},
	{
		name:        "Back to the menu",
		description: "The host quits to the main menu mid-round; everyone is unmuted at once.",
		steps: concat(openLobby(), startTasks(), []step{
			phase(game.MENU),
			expect("You are unmuted and undeafened immediately, and the status message reads Menu with no players listed."),
			phase(game.LOBBY),
			expect("The status message returns to the Lobby view."),
		}),
	},
	{
		name:        "Impostor victory",
		description: "You are the impostor and win by sabotage.",
		steps: concat(openLobby(), startTasks(), []step{
			player(mockGreen, game.DIED, game.Green, false),
			phase(game.DISCUSS),
			expect("You are unmuted and undeafened, and the status message shows Mock Green as dead."),
			phase(game.TASKS),
			expect("After about 7 seconds you are muted and deafened."),
			// Roles are only reported at gameover; re-sending the player marks you as the impostor.
			player(self, game.FORCEUPDATED, game.Red, true),
			gameover(game.ImpostorBySabotage),
			expect("A match summary is posted saying impostors won by sabotage and mentioning you as winning as Imposter, and you are unmuted and undeafened within about a second."),
		}),
	},
	{
		name:        "Lobby changes",
		description: "Color changes, a player leaving, and a new lobby code while still in the lobby; no linking needed.",
		steps: []step{
			lobby("TESTCODE", game.NA, game.SKELD),
			player(self, game.JOINED, game.Red, false),
			player(mockBlue, game.JOINED, game.Blue, false),
			player(mockGreen, game.JOINED, game.Green, false),
			expect("The status message shows lobby code TESTCODE on The Skeld with {you} (red), Mock Blue and Mock Green."),
			player(self, game.CHANGECOLOR, game.Cyan, false),
			expect("{you} is now shown as cyan."),
			player(mockGreen, game.LEFT, game.Green, false),
			expect("Mock Green is no longer listed."),
			lobby("NEWCODE", game.EU, game.POLUS),
			expect("The status message shows lobby code NEWCODE, region Europe and map Polus."),
		},
	},
}

func concat(parts ...[]step) []step {
	var steps []step
	for _, part := range parts {
		steps = append(steps, part...)
	}
	return steps
}

// scenarioRun plays a scenario one checkpoint at a time so a UI can wait
// for the operator's answer between calls.
type scenarioRun struct {
	sc      scenario
	s       *session
	you     string
	next    int
	results []checkResult
}

func newRun(s *session, sc scenario, you string) *scenarioRun {
	return &scenarioRun{sc: sc, s: s, you: you}
}

func (r *scenarioRun) fill(text string) string { return strings.ReplaceAll(text, self, r.you) }

// advance sends steps until the next checkpoint and returns what it did, with
// the checkpoint text; an empty checkpoint means the scenario has finished. A
// failed send stops immediately.
func (r *scenarioRun) advance() (entries []logEntry, checkpoint string, err error) {
	for r.next < len(r.sc.steps) {
		st := r.sc.steps[r.next]
		switch {
		case st.send != nil:
			if err := st.send(r.s, r.you); err != nil {
				return entries, "", err
			}
			entries = append(entries, logEntry{entrySent, r.fill(st.label)})
		case st.expect != "":
			return entries, r.fill(st.expect), nil
		default:
			entries = append(entries, logEntry{entryNote, r.fill(st.label)})
		}
		r.next++
	}
	return entries, "", nil
}

// progress reports how many checkpoints have been answered, out of all of them.
func (r *scenarioRun) progress() (answered, total int) {
	for _, st := range r.sc.steps {
		if st.expect != "" {
			total++
		}
	}
	return len(r.results), total
}

// answer records the operator's answer at the current checkpoint.
func (r *scenarioRun) answer(observed bool) {
	r.results = append(r.results, checkResult{r.fill(r.sc.steps[r.next].expect), observed})
	r.next++
}

// abort sends the bot back to the menu, so nobody is left muted by a
// half-played round, and reports errAborted.
func (r *scenarioRun) abort() error {
	if err := r.s.phase(game.MENU); err != nil {
		return err
	}
	return errAborted
}

func tally(results []checkResult) (observed int) {
	for _, r := range results {
		if r.observed {
			observed++
		}
	}
	return observed
}
