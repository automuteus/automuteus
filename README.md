<p align="center">
    <a href="https://automute.us/#/" alt = "Website link"><img src="assets/AutoMuteUsBanner_cropped.png" width="800"></a>
</p>
<p align="center">
    <a href="https://github.com/automuteus/automuteus/actions?query=build" alt="Build Status">
        <img src="https://github.com/automuteus/automuteus/workflows/build/badge.svg" />
    </a>
    <a href="https://github.com/automuteus/automuteus/releases/latest">
    <img alt="GitHub release" src="https://img.shields.io/github/v/release/automuteus/automuteus" >
    </a>
    <a href="https://github.com/automuteus/automuteus/graphs/contributors" alt="Contributors">
        <img src="https://img.shields.io/github/contributors/automuteus/automuteus" />
    </a>
    <a href="https://discord.gg/ZkqZSWF" alt="Discord Link">
        <img src="https://img.shields.io/discord/754465589958803548?logo=discord" />
    </a>
</p>
<p align="center">
    <a href="https://hub.docker.com/repository/docker/automuteus/automuteus" alt="Pulls">
        <img src="https://img.shields.io/docker/pulls/denverquane/amongusdiscord.svg" />
    </a>
    <a href="https://hub.docker.com/repository/docker/automuteus/automuteus" alt="Stars">
        <img src="https://img.shields.io/docker/stars/denverquane/amongusdiscord.svg" />
    </a>
    <a href="https://goreportcard.com/report/github.com/automuteus/automuteus" alt="Report Card">
        <img src="https://goreportcard.com/badge/github.com/automuteus/automuteus" />
    </a>
</p>

<p align="center">
    <a href="https://add.automute.us" alt="invite">
        <img alt="Invite Link" src="https://img.shields.io/static/v1?label=bot&message=invite%20me&color=purple">
    </a>
</p>

# AutoMuteUs

<div style="display: flex; align-item: center; justify: center;">
<p style="">
    <a href="https://add.automute.us"/>
        <img src="assets/DiscordBot_Black.gif", width=150>
    </a>
</p>
<div style="margin-left: 2%">
AutoMuteUs is a Discord Bot to harness Among Us game data, and automatically mute/unmute players during games!

Requires [amonguscapture](https://github.com/automuteus/amonguscapture) to capture and relay game data.

Have any questions, concerns, bug reports, or just want to chat? Join our discord at https://discord.gg/ZkqZSWF!

Click the "invite me" badge in the header to invite the bot to your server, or click the GIF on the left.

All artwork for the bot has been generously provided by <a href=https://aspen-cyborg.tumblr.com/>Smiles</a>!


</div>
</div>

# ⚠️ Requirements ⚠️

1. You **must** run the [Capture application](https://github.com/automuteus/amonguscapture/releases/latest) on your
   Windows PC for the bot to work! Any Among Us games that don't have a user running the capture software will **not
   have automuting capabilities**!
2. The [Capture application](https://github.com/automuteus/amonguscapture/releases) currently only supports the Steam,
   Epic Games, itch.io, and Microsoft Store releases of the game, but **does not** support beta or cracked versions.

# Quickstart and Demo (click the image):

[![Quickstart](http://i3.ytimg.com/vi/VYx6kM1O4FM/hqdefault.jpg)](https://youtu.be/VYx6kM1O4FM)

# Usage and Commands

To start a bot game in the current channel, type the following slash command in Discord after inviting the bot:

```
/new
# Starts a game, and allows users to react to emojis to link to their in-game players
```

The bot will send you a private reply with a link that is used to sync the capture software to your game. It will also have a link to download the latest version of the capture software, if you don't have it already.

If you want to view command usage or see the available options, type `/help` in your Discord channel.

## Commands

| Command        | Arguments      | Description                                                                                                            | Example            |
|----------------|----------------|------------------------------------------------------------------------------------------------------------------------|--------------------|
| `/help`        | None           | Print help info and command usage                                                                                      |                    |
| `/new`         | None           | Start a new game in the current text channel                                                                           |                    |
| `/refresh`     | None           | Remake the bot's status message entirely, in case it ends up too far up in the chat.                                   |                    |
| `/pause`       | None           | Pause the bot, and don't let it automute anyone until unpaused.                                                        |                    |
| `/end`         | None           | End the game entirely, and stop tracking players. Unmutes all and resets state                                         |                    |
| `/link`        | @name color    | Manually link a discord user to their in-game color                                                                    | `/link @Soup cyan` |
| `/unlink`      | @name          | Manually unlink a player                                                                                               | `./unlink @player` |
| `.au settings` |                | View and change settings for the bot, such as the command prefix or mute behavior                                      |                    |
| `/privacy`     |                | View privacy and data collection information about the bot                                                             |                    |
| `/info`        | None           | View general info about the Bot                                                                                        |                    |
| `/map`         | MAPNAME        | View an image of an in-game map in the text channel. Provide the name of the map, and if you want the detailed version | `/map skeld true`  |
| `.au stats`    | guild or @name | View detailed stats about Among Us games played on the current server, or by a specific player                         | `.au stats @Soup`  |
| `.au premium`  |                | View information about AutoMuteUs Premium, and the current premium status of your server                               |                    |

_In addition to handful of more secretive Easter Egg commands..._

# Privacy

You can view privacy and data collection details for the Official Bot [here](PRIVACY.md).

# Localization

View details on Localization and Multi-Language support [here](LOCALIZATION.md).

# Self-Hosting

Self-hosting requires robust knowledge and troubleshooting capability for Docker/Docker-compose, unRAID, Heroku, and/or any other networking and routing config specific to your hosting solution.

As such, **we recommend that the majority of users take advantage of our Verified bot**. The link to invite our bot can be found here:

<a href="https://add.automute.us" alt="invite">
        <img alt="Invite Link" src="https://img.shields.io/static/v1?label=bot&message=invite%20me&color=purple">
    </a>

If you are certain that you would prefer to self-host the bot, please follow any of the instructions on [automuteus/deploy](https://github.com/automuteus/deploy).

# Developing

Please refer to the instructions on [automuteus/deploy](https://github.com/automuteus/deploy).

# Similar Projects

- [Imposter](https://github.com/molenzwiebel/Impostor): Similar bot that uses private Discord channels instead of mute/deafen. Also uses a dummy player joining the game and "spectating" to get game information; no capture needed (although loses the 10th player slot).

- [AmongUsBot](https://github.com/alpharaoh/AmongUsBot): Without their original Python program
  with a lot of the OCR/Discord functionality, I never would have even thought of this idea! **Not currently maintained**

- [amongcord](https://github.com/pedrofracassi/amongcord): A great program for tracking player status and auto mute/unmute in Among Us.
  Their project works like a traditional Discord bot; very easy installation!

- [Silence Among Us](https://github.com/tanndev/silence-among-us#silence-among-us): Another bot quite similar to this one, which also uses AmongUsCapture. Now in early-access with a publicly-hosted instance!
