# Capture mock

Interactive development client that plays scripted game scenarios through
Galactus over Socket.IO, using the same game types and event names as the bot
and broker, and asks you to confirm what the bot did in Discord at each step.
Migrated from [automuteus/capture-mock](https://github.com/automuteus/capture-mock).

From the repository root:

```sh
go run ./cmd/capture-mock
```

Start your development bot and Galactus with their shared Redis instance, create
a game with `/new`, and paste its **connect code** or `aucapture://` link at the
prompt. The connect code is distinct from the Among Us lobby code.

The broker is intentionally fixed to `http://localhost:8123` in source. There is
no environment variable or command-line override. Capture links must use
`aucapture://localhost:8123/CODE?insecure`; remote hosts, other ports, and secure
links are rejected. This adds friction against using the mock on live games.

Enter the name of the player you will link to. Using your own Discord username
or nickname links you automatically when that player joins; any other name can be
linked from the status message's color dropdown when a scenario tells you to.

Then pick a scenario by number. Each scenario sends a scripted sequence of events
and pauses at checkpoints (`??` lines) describing what the bot should have done
in Discord: which players the status message lists, whether you are muted or
deafened, what the match summary says. Answer `y` or `n` for each; the answers
are tallied in a report when the scenario ends. Answer `q` to stop a scenario
early; the mock then sends a menu phase so nobody is left muted. Every scenario
ends back in the lobby, so they can be run in any order and repeated.

| Scenario | Exercises |
| --- | --- |
| Full round | Lobby, tasks, discussion, tasks, crewmate win; alive mute and deafen rules and phase delays. |
| Killed during tasks | Dead player rules: no status edit during tasks, muted but not deafened in discussion, free during tasks. |
| Exiled at a meeting | Exile updates the status message at once but defers voice changes to the next phase. |
| Leaving mid-round | A player leaves during tasks and another disconnects at a meeting. |
| Back to the menu | Quitting to the main menu unmutes everyone immediately. |
| Impostor victory | Role reporting at gameover; the summary names you as the winning impostor. |
| Lobby changes | Color changes, leaving, and a new lobby code while still in the lobby; no linking needed. |

Checkpoint text describes the default voice rules and phase delays; guild
settings that change them (unmute dead during tasks, custom delays, muting
spectators) will change what you see. The scenarios live in `scenario.go`;
add one by composing the `lobby`, `phase`, `player`, `gameover`, `expect` and
`note` helpers, and the tests check that it plays through and ends in the lobby.

Choose `M` to send events manually instead: `L` sends lobby details, `S` a
phase, `P` a player event, `G` gameover, and `Q` returns to the scenario menu.
Numbered menus use the current game definitions; blank answers select the
displayed defaults. Player names can contain spaces. EOF or Ctrl+C exits the
program. Input errors are retried; failed sends stop the client.

Lobby and gameover each send a following lobby-phase event. Player events collect
participants and their impostor roles for gameover, including departed players.
Sending lobby or gameover resets that list, so send player events for each new
round. Re-entering a player name updates their role without duplicating them.

This simulates game events only: it does not read Among Us memory, join Discord,
or execute capture-side mute/deafen tasks. A successful send is not confirmation
that the bot processed the event; inspect the bot and Galactus logs as needed.

```sh
go test ./cmd/capture-mock ./pkg/capture
go build ./...
```
