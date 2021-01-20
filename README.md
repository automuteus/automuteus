<p align="center">
    <a href="https://automute.us/#/" alt = "Website link"><img src="assets/AutoMuteUsBanner_cropped.png" width="800"></a>
</p>
<p align="center">
    <a href="https://github.com/denverquane/amongusdiscord/actions?query=build" alt="Build Status">
        <img src="https://github.com/denverquane/amongusdiscord/workflows/build/badge.svg" />
    </a>
    <a href="https://github.com/denverquane/automuteus/releases/latest">
    <img alt="GitHub release" src="https://img.shields.io/github/v/release/denverquane/automuteus" >
    </a>
    <a href="https://github.com/denverquane/amongusdiscord/graphs/contributors" alt="Contributors">
        <img src="https://img.shields.io/github/contributors/denverquane/amongusdiscord" />
    </a>
    <a href="https://discord.gg/ZkqZSWF" alt="Discord Link">
        <img src="https://img.shields.io/discord/754465589958803548?logo=discord" />
    </a>
</p>
<p align="center">
    <a href="https://hub.docker.com/repository/docker/denverquane/amongusdiscord" alt="Pulls">
        <img src="https://img.shields.io/docker/pulls/denverquane/amongusdiscord.svg" />
    </a>
    <a href="https://hub.docker.com/repository/docker/denverquane/amongusdiscord" alt="Stars">
        <img src="https://img.shields.io/docker/stars/denverquane/amongusdiscord.svg" />
    </a>
    <a href="https://goreportcard.com/report/github.com/denverquane/automuteus" alt="Report Card">
        <img src="https://goreportcard.com/badge/github.com/denverquane/automuteus" />
    </a>
</p>

<p align="center">
    <a href="https://discord.com/api/oauth2/authorize?client_id=753795015830011944&permissions=267746384&scope=bot" alt="invite">
        <img alt="Invite Link" src="https://img.shields.io/static/v1?label=bot&message=invite%20me&color=purple">
    </a>
</p>

# AutoMuteUs

<div style="display: flex; align-item: center; justify: center;">
<p style="">
    <a href="https://discord.com/api/oauth2/authorize?client_id=753795015830011944&permissions=267746384&scope=bot"/>
        <img src="assets/DiscordBot_Black.gif", width=150>
    </a>
</p>
<div style="margin-left: 2%">
AutoMuteUs is a Discord Bot to harness Among Us game data, and automatically mute/unmute players during games!

Requires [amonguscapture](https://github.com/automuteus/amonguscapture) to capture and relay game data.

Have any questions, concerns, bug reports, or just want to chat? Join our discord at https://discord.gg/ZkqZSWF!

Click the "invite me" badge in the header to invite the bot to your server, or click the GIF above.

All artwork for the bot has been generously provided by <a href=https://aspen-cyborg.tumblr.com/>Smiles</a>!


</div>
</div>

# ⚠️ Requirements ⚠️

1. You **must** run the [Capture application](https://github.com/automuteus/amonguscapture/releases/latest) on your Windows PC for the bot to work! Any Among Us games that don't have a user running the capture software will **not have automuting capabilities**!
2. The [Capture application](https://github.com/automuteus/amonguscapture/releases) currently only supports the **Non-Beta Official Steam** release of the game.

# Quickstart and Demo (click the image):

[![Quickstart](https://img.youtube.com/vi/kO4cqMKV2yI/0.jpg)](https://youtu.be/kO4cqMKV2yI)

# Usage and Commands

To start a bot game in the current channel, type the following `.au` command in Discord after inviting the bot:

```
.au new
# Starts a game, and allows users to react to emojis to link to their in-game players
```

The bot will send you a private message (make sure your Discord settings allow DMs from server members!) with a link that is used to sync the capture software to your game. It will also have a link to download the latest version of the capture software, if you don't have it already.

If you want to view command usage or see the available options, type `.au` or `.au help` in your Discord channel.

## Commands

The Discord Bot uses the `.au` prefix for any commands by default; if you change your prefix remember to replace `.au` with your custom prefix. If you forget your prefix, you can @mention the bot and it will respond with whatever it's prefix currently is.

| Command        | Alias   | Arguments   | Description                                                                                                     | Example                            |
| -------------- | ------- | ----------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| `.au help`     | `.au h` | None        | Print help info and command usage                                                                               |                                    |
| `.au new`      | `.au n` | None        | Start a new game in the current text channel. Optionally accepts the room code and region                       | `.au n CODE eu`                    |
| `.au link`     | `.au l` | @name color | Manually link a discord user to their in-game color                                                             | `.au l @Soup cyan`                 |
| `.au refresh`  | `.au r` | None        | Remake the bot's status message entirely, in case it ends up too far up in the chat.                            |                                    |
| `.au end`      | `.au e` | None        | End the game entirely, and stop tracking players. Unmutes all and resets state                                  |                                    |
| `.au unlink`   | `.au u` | @name       | Manually unlink a player                                                                                        | `.au u @player`                    |
| `.au settings` | `.au s` |             | View and change settings for the bot, such as the command prefix or mute behavior                               |                                    |
| `.au pause`    | `.au p` | None        | Pause the bot, and don't let it automute anyone until unpaused. **will not un-mute muted players, be careful!** |                                    |
| `.au privacy`  |         |             | View privacy and data collection information about the bot                                                      |                                    |
| `.au info`     | `.au i` | None        | View general info about the Bot                                                                                 |                                    |

_In addition to handful of more secretive Easter Egg commands..._

# Privacy

You can view privacy and data collection details for the Official Bot [here](PRIVACY.md).

# Localization

View details on Localization and Multi-Language support [here](LOCALIZATION.md).

# Self-Hosting

If you would prefer to self-host the bot, the steps for doing so are provided below.
Self-hosting requires robust knowledge and troubleshooting capability for Docker/Docker-compose, unRAID, Heroku, and/or any other networking and routing config specific to your hosting solution.

As such, **we recommend that the majority of users take advantage of our Verified bot**. The link to invite our bot can be found here:

<a href="https://discord.com/api/oauth2/authorize?client_id=753795015830011944&permissions=267746384&scope=bot" alt="invite">
        <img alt="Invite Link" src="https://img.shields.io/static/v1?label=bot&message=invite%20me&color=purple">
    </a>

If you are certain that you would prefer to self-host the bot, please refer to the documentation [here](https://github.com/automuteus/deploy).

# Similar Projects

- [Imposter](https://github.com/molenzwiebel/Impostor): Similar bot that uses private Discord channels instead of mute/deafen. Also uses a dummy player joining the game and "spectating" to get game information; no capture needed (although loses the 10th player slot).

- [AmongUsBot](https://github.com/alpharaoh/AmongUsBot): Without their original Python program
  with a lot of the OCR/Discord functionality, I never would have even thought of this idea! **Not currently maintained**

- [amongcord](https://github.com/pedrofracassi/amongcord): A great program for tracking player status and auto mute/unmute in Among Us.
  Their project works like a traditional Discord bot; very easy installation!

- [Silence Among Us](https://github.com/tanndev/silence-among-us#silence-among-us): Another bot quite similar to this one, which also uses AmongUsCapture. Now in early-access with a publicly-hosted instance!
