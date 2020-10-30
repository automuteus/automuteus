<p align="center">
    <img src="assets/AutoMuteUsBanner_cropped.png" width="800">
</p>
<p align="center">
    Banner and Bot profile picture courtesy of <a href=https://aspen-cyborg.tumblr.com/>Smiles</a>!
</p>

<p align="center">
    <a href="https://github.com/denverquane/amongusdiscord/actions?query=build" alt="Build Status">
        <img src="https://github.com/denverquane/amongusdiscord/workflows/build/badge.svg" />
    </a>
    <a href="https://github.com/denverquane/amongusdiscord/graphs/contributors" alt="Contributors">
        <img src="https://img.shields.io/github/contributors/denverquane/amongusdiscord" />
    </a>
    <a href="https://discord.gg/ZkqZSWF" alt="Discord Link">
        <img src="https://img.shields.io/discord/754465589958803548?logo=discord" />
    </a>
</p>
<p align="center">
    <a href="pulls" alt="Pulls">
        <img src="https://img.shields.io/docker/pulls/denverquane/amongusdiscord.svg" />
    </a>
    <a href="stars" alt="Stars">
        <img src="https://img.shields.io/docker/stars/denverquane/amongusdiscord.svg" />
    </a>
</p>
<p align="center">
    <img alt="GitHub release" src="https://img.shields.io/github/v/release/denverquane/automuteus">
</p>

<p align="center">
    <a href="https://discord.com/api/oauth2/authorize?client_id=753795015830011944&permissions=267746384&scope=bot"/>
        <img src="https://github.com/denverquane/automuteus/blob/master/assets/BotProfilePicture.png?raw=true", width=100>
    </p>
</p>
<p align="center">
    <a href="https://discord.com/api/oauth2/authorize?client_id=753795015830011944&permissions=267746384&scope=bot" alt="invite">
        <img alt="Invite Link" src="https://img.shields.io/static/v1?label=bot&message=invite%20me&color=purple">
    </a>
</p>

# AutoMuteUs (BETA)

Discord Bot to harness Among Us game data, and automatically mute/unmute players during the course of the game!

Works in conjunction with [amonguscapture](https://github.com/denverquane/amonguscapture)

**This program is in Beta. While we are confident about the basic functionality, there will still be issues or pecularities with how the program functions! We are actively working to resolve these issues!**

Have any questions, concerns, bug reports, or just want to chat? Join the discord at https://discord.gg/ZkqZSWF!

# ⚠️ Basic Requirements ⚠️

1. The Capture application only supports the **Non-Beta Official Steam** release of the game
2. The Capture application must be run on a Windows PC. The program **CANNOT** be run directly on mobile phones, so you will need 1 or more players on PC in every match.

# Quickstart and Demo (click the image):

[![Installation](https://img.youtube.com/vi/LUptOv5ohNc/0.jpg)](https://youtu.be/LUptOv5ohNc)

# Usage and Commands

To start a bot game in the current channel, type the following `.au` command in Discord after inviting the bot:

```
.au new
# Starts a game, and allows users to react to emojis to link to their in-game players
```

Get Playing!

If you want to view command usage or see the available options, type `.au` or `.au help` in your Discord channel.

## Commands

The Discord Bot uses the `.au` prefix for any commands

| Command        | Alias   | Arguments   | Description                                                                                                     | Example                            |
| -------------- | ------- | ----------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| `.au help`     | `.au h` | None        | Print help info and command usage                                                                               |                                    |
| `.au new`      | `.au n` | None        | Start a new game in the current text channel. Optionally accepts the room code and region                       | `.au n CODE eu`                    |
| `.au track`    | `.au t` | VC Name     | Tell Bot to use a single voice channel for mute/unmute, and ignore any other channels                           | `.au t Test Voice`                 |
| `.au link`     | `.au l` | @name color | Manually link a discord user to their in-game color                                                             | `.au l @Soup cyan`                 |
| `.au refresh`  | `.au r` | None        | Remake the bot's status message entirely, in case it ends up too far up in the chat.                            |                                    |
| `.au end`      | `.au e` | None        | End the game entirely, and stop tracking players. Unmutes all and resets state                                  |                                    |
| `.au unlink`   | `.au u` | @name       | Manually unlink a player                                                                                        | `.au u @player`                    |
| `.au settings` | `.au s` |             | View and change settings for the bot, such as the command prefix or mute behavior                               |                                    |
| `.au force`    | `.au f` | stage       | Force a transition to a stage if you encounter a problem in the state                                           | `.au f task` or `.au f d`(discuss) |
| `.au pause`    | `.au p` |             | Pause the bot, and don't let it automute anyone until unpaused. **will not un-mute muted players, be careful!** |                                    |
| `.au log`      |         | message     | Issue a small log message that will help you find the message later, if a problem occurs                        | `.au log Something bad happened`   |

*In addition to handful of more secretive Easter Egg commands...*

# Self-Hosting

If you would prefer to self-host the bot, the steps for doing so are provided below. 
Self-hosting requires robust knowledge and troubleshooting capability for Docker/Docker-compose, unRAID, Heroku, and any other networking and routing config specific to your hosting solution. 

As such, **we recommend that the majority of users take advantage of our Verified bot**. The link to invite our bot can be found here:     

<a href="https://discord.com/api/oauth2/authorize?client_id=753795015830011944&permissions=267746384&scope=bot" alt="invite">
        <img alt="Invite Link" src="https://img.shields.io/static/v1?label=bot&message=invite%20me&color=purple">
    </a>

If you are certain that you would prefer to self-host the bot, please follow any of the guides detailed below.

## Pre-Installation Steps, Important!

- Create an Application and Bot account (requires Admin privileges on the Server in question). [Instructions here](https://github.com/denverquane/amongusdiscord/blob/master/BOT_README.md)

Now follow any of the specific hosting options provided below:

## Docker Compose:

Docker compose is the simplest and recommended method for self-hosting AutoMuteUs, but it does require an existing physical machine or VPS to run on.

There is a `docker-compose.yml` file in this repository that will provide all the consituent components to run AutoMuteUs.

### Steps:
- Install Docker and Docker Compose on the machine you will be using to host AutoMuteUs
- Download the `docker-compose.yml` from this repository, and create a `.env` file in the same directory that will contain your Environment Variables. On Linux/UNIX systems you can use `touch .env` to create this file, but a template `sample.env` is provided in this repository for reference.
- Provide your specific Environment Variables in the `.env` file, as relevant to your configuration. Please see the Environment Variables reference further down in this Readme for details, as well as the `sample.env` provided.
- Run `docker-compose pull`. This will download the latest built Docker images from Dockerhub that are required to run AutoMuteUs.
- Run `docker-compose up -d` to start all the containers required for AutoMuteUs to function. The containers will now be running in the background, but you can view the logs for the containers using `docker-compose logs`, or `docker-compose logs -f` to follow along as new log entries are generated.

## unRAID

unRAID hosting steps are are not updated for v3.0+ of AutoMuteUs, and as such is not supported at this time.

## Heroku

Heroku hosting steps are are not updated for v3.0+ of AutoMuteUs, and as such is not supported at this time.

## Environment Variables

### Required

- `DISCORD_BOT_TOKEN`: The Bot Token used by the bot to authenticate with Discord.
- `REDIS_ADDRESS`: The host and port at which your Redis database instance is accessible. Ex: `192.168.1.42:6379`

### Optional

- `DISCORD_BOT_TOKEN_2`: A second Bot Token to be used to distribute the mute/deafen requests to Discord.
  If you play in larger groups of 8+ people, this is recommended to not be rate-limited (delayed) by Discord when rounds change!
- `EMOJI_GUILD_ID`: If your bot is a member of multiple guilds, this ID can be used to specify the single guild that it should use for emojis (no need to add the emojis to ALL servers).
- `PORT`: The **internal** port the Bot will use for incoming socket.io communications. Defaults to 8123.
- `HOST`: The **externally-accessible URL** for the discord bot. For example, `http://test.com:8123`.
  This is used to provide the linking URI to the capture, via the Direct Message the bot sends you when typing `.au new`.
  **You must specify `http://` or `https://` accordingly, and specify the port if non-8123. For example, `https://your-app.herokuapp.com:443`**
- `SERVICE_PORT`: Port used for graceful shutdowns and stats via HTTP GET. Defaults to 5000
- `CONFIG_PATH`: Alternate filesystem path for guild and user config files. Defaults to `./`
- `LOG_PATH`: Filesystem path for log files. Defaults to `./`
- `CAPTURE_TIMEOUT`: How many seconds of no capture events received before the Bot will terminate the associated game/connection. Defaults to 36000 seconds.

### HIGHLY advanced. Probably don't ever touch these!

- `NUM_SHARDS`: Num shards provided to the Discord API.
- `SHARD_ID`: Shard ID used to identify with the Discord API. Needs to be strictly less than `NUM_SHARDS`

# Similar Projects

- [Imposter](https://github.com/molenzwiebel/Impostor): Similar bot that uses private Discord channels instead of mute/deafen. Also uses a dummy player joining the game and "spectating" to get game information; no capture needed (although loses the 10th player slot).

- [AmongUsBot](https://github.com/alpharaoh/AmongUsBot): Without their original Python program
  with a lot of the OCR/Discord functionality, I never would have even thought of this idea! **Not currently maintained**

- [amongcord](https://github.com/pedrofracassi/amongcord): A great program for tracking player status and auto mute/unmute in Among Us.
  Their project works like a traditional Discord bot; very easy installation!

- [Silence Among Us](https://github.com/tanndev/silence-among-us#silence-among-us): Another bot quite similar to this one, which also uses AmongUsCapture. Now in early-access with a publicly-hosted instance!

# Troubleshooting

- **"Websocket 400-something: Authentication Failed" Error!**
  Your `DISCORD_BOT_TOKEN` is incorrect or invalid. Make sure you copied/pasted the Bot _token_, NOT the "client secret" from the Discord Developer portal

- **"Emoji ID is not a snowflake" Error! Or the bot doesn't provide emojis as reactions on the status message!**
  The discord API is agonizingly slow to upload new emojis, inform bots about the presence of new/updated emojis, and delete emojis.
  The easiest answer is to **give it a while** (sometimes can take almost 30 minutes), and try again.
