# Capture mock

Interactive development client that sends capture events through Galactus over
Socket.IO, using the same game types and event names as the bot and broker.
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

Choose `L` to send lobby details, `S` for a phase, `P` for a player event, `G` for
gameover, or `Q` to quit. Numbered menus use the current game definitions; blank
answers select the displayed defaults. Player names can contain spaces. EOF or
Ctrl+C exits the program. Input errors are retried; failed sends stop the client.

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
