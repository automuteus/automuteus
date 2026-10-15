# AutoMuteUs

A Discord bot that mutes and unmutes Among Us players as a game moves between phases, driven by game events a Capture reports.

## Language

### Captures

**Capture**:
Whatever is connected to Galactus under a Connect Code and reporting game events for one Among Us player's game. The bot treats every Capture the same.
_Avoid_: capture client, the capture (when meaning the app)

**Capture App**:
AmongUsCapture, the standalone program that reads game state from outside the Among Us process.
_Avoid_: the capture, AUCapture

**Capture Mod**:
A BepInEx plugin, loaded inside a modded Among Us, that acts as a Capture. An officially supported alternative to the Capture App, not its replacement.
_Avoid_: plugin, capture plugin

**Capture Kind**:
Whether a Capture is the Capture App or the Capture Mod. Recorded per match alongside the Capture's version; unrecorded means a Capture App older than when Captures began reporting it.
_Avoid_: client type, client

**Connect Code**:
The 8-character code that ties a Capture to one bot game. At most one Capture holds a Connect Code at a time; a newly connecting Capture replaces the one holding it.

**Capture Link**:
The `aucapture://host/CODE` link the bot posts for a new game, carrying Galactus' address and the Connect Code.
_Avoid_: connect link, capture URL

### Games

**Unknown Ending**:
A game end the Capture can't name, such as a role mod's custom win. Neither side won, so it doesn't count toward stats.
_Avoid_: unknown win, crew win (for these)
