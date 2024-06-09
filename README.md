<p align="center">
    <a href="https://automute.us/#/" alt = "Website link"><img src="assets/AutoMuteUsBanner_cropped.png" width="800"></a>
</p>
<p align="center">
    <a href="https://github.com/j0nas500/automuteus-tor/actions?query=build" alt="Build Status">
        <img src="https://github.com/j0nas500/automuteus-tor/workflows/build/badge.svg" />
    </a>
    <a href="https://github.com/j0nas500/automuteus-tor/releases/latest">
    <img alt="GitHub release" src="https://img.shields.io/github/v/release/automuteus/automuteus" >
    </a>
    <a href="https://github.com/j0nas500/automuteus-tor/graphs/contributors" alt="Contributors">
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
    <a href="https://automuteus.crowdin.com/automuteus" alt="localize">
        <img alt="Localize" src="https://badges.crowdin.net/e/5eb1365b5fd16082e63cc54c33736adc/localized.svg">
    </a>
    <a href="https://goreportcard.com/report/github.com/j0nas500/automuteus-tor" alt="Report Card">
        <img src="https://goreportcard.com/badge/github.com/j0nas500/automuteus-tor" />
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

# HOW TO

## Public Bot

1. Invite the Public [Bot](https://discord.com/oauth2/authorize?client_id=762435324953231440&scope=bot%20applications.commands&permissions=12854272)
2. Use this custom [AmongUsCapture.exe](https://github.com/j0nas500/amonguscapture/releases)
3. Maybe you need also [.NET Desktop Runtime 5.0.1](https://download.visualstudio.microsoft.com/download/pr/c6a74d6b-576c-4ab0-bf55-d46d45610730/f70d2252c9f452c2eb679b8041846466/windowsdesktop-runtime-5.0.1-win-x64.exe)

## Self Host with Docker Compose

1. Use this `docker-compose.yml`:
```
version: "3"

services:
  automuteus:
    # Either:
    # - Use a prebuilt image
    #image: automuteus/automuteus:${AUTOMUTEUS_TAG:?err}
    # - Use an old prebuilt image (prior to 6.16.1)
    #image: denverquane/amongusdiscord:${AUTOMUTEUS_TAG:?err}
    # - Build image from local source
    #build: ../automuteus-tor
    # - Build image from github directly
    build: https://github.com/j0nas500/automuteus-tor.git

    restart: always
    ports:
      # 5000 is the default service port
      # Format is HostPort:ContainerPort
      - ${SERVICE_PORT:-5000}:5000
    environment:
      # These are required and will fail if not present
      - DISCORD_BOT_TOKEN=${DISCORD_BOT_TOKEN:?err}
      - HOST=${GALACTUS_HOST:?err}
      - POSTGRES_USER=${POSTGRES_USER:?err}
      - POSTGRES_PASS=${POSTGRES_PASS:?err}

      # These Variables are optional
      - EMOJI_GUILD_ID=${EMOJI_GUILD_ID:-}
      - CAPTURE_TIMEOUT=${CAPTURE_TIMEOUT:-}
      - AUTOMUTEUS_LISTENING=${AUTOMUTEUS_LISTENING:-}
      - AUTOMUTEUS_GLOBAL_PREFIX=${AUTOMUTEUS_GLOBAL_PREFIX:-}
      - BASE_MAP_URL=${BASE_MAP_URL:-}
      - SLASH_COMMAND_GUILD_IDS=${SLASH_COMMAND_GUILD_IDS:-}

      # Do **NOT** change this
      - REDIS_ADDR=${AUTOMUTEUS_REDIS_ADDR}
      - GALACTUS_ADDR=${GALACTUS_ADDR}
      - POSTGRES_ADDR=${POSTGRES_ADDR}
    stop_grace_period: ${STOP_GRACE_PERIOD:-2m}
    depends_on:
      - redis
      - galactus
      - postgres
    volumes:
      - "bot-logs:/app/logs"
  galactus:
    ports:
      # See sample.env for details, but in general, match the GALACTUS_EXTERNAL_PORT w/ the GALACTUS_HOST's port
      - ${GALACTUS_EXTERNAL_PORT:-8123}:${BROKER_PORT}
    image: automuteus/galactus:${GALACTUS_TAG:?err}
    restart: always
    environment:
      # This Variable is optional
      - WORKER_BOT_TOKENS=${WORKER_BOT_TOKENS:-}

      # Do **NOT** change these
      - DISCORD_BOT_TOKEN=${DISCORD_BOT_TOKEN:?err}
      - BROKER_PORT=${BROKER_PORT}
      - REDIS_ADDR=${GALACTUS_REDIS_ADDR}
      - GALACTUS_PORT=${GALACTUS_PORT}
    depends_on:
      - redis

  redis:
    image: redis:alpine
    restart: always
    volumes:
      - "redis-data:/data"

  postgres:
    image: postgres:12-alpine
    restart: always
    environment:
      - POSTGRES_USER=${POSTGRES_USER}
      - POSTGRES_PASSWORD=${POSTGRES_PASS}
    volumes:
      - "postgres-data:/var/lib/postgresql/data"

volumes:
  bot-logs:
  redis-data:
  postgres-data:
```
2. Fill the Variables with the [`sample.env`](https://github.com/automuteus/deploy/blob/main/sample.env)
3. Use this custom [AmongUsCapture.exe](https://github.com/j0nas500/amonguscapture/releases)
4. Maybe you need also [.NET Desktop Runtime 5.0.1](https://download.visualstudio.microsoft.com/download/pr/c6a74d6b-576c-4ab0-bf55-d46d45610730/f70d2252c9f452c2eb679b8041846466/windowsdesktop-runtime-5.0.1-win-x64.exe)


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

| Command     | Description                                                                                                            | Example                  |
|-------------|------------------------------------------------------------------------------------------------------------------------|--------------------------|
| `/help`     | Print help info and command usage                                                                                      |                          |
| `/new`      | Start a new game in the current text channel                                                                           |                          |
| `/refresh`  | Remake the bot's status message entirely, in case it ends up too far up in the chat.                                   |                          |
| `/pause`    | Pause the bot, and don't let it automute anyone until unpaused.                                                        |                          |
| `/end`      | End the game entirely, and stop tracking players. Unmutes all and resets state                                         |                          |
| `/link`     | Manually link a discord user to their in-game color                                                                    | `/link @Soup cyan`       |
| `/unlink`   | Manually unlink a player                                                                                               | `/unlink @Soup`          |
| `/settings` | View and change settings for the bot, such as the command prefix or mute behavior                                      |                          |
| `/privacy`  | View privacy and data collection information about the bot                                                             |                          |
| `/info`     | View general info about the Bot                                                                                        |                          |
| `/map`      | View an image of an in-game map in the text channel. Provide the name of the map, and if you want the detailed version | `/map skeld true`        |
| `/stats`    | View detailed stats about Among Us games played on the current server, or by a specific player                         | `/stats user view @Soup` |
| `/premium`  | View information about AutoMuteUs Premium, and the current premium status of your server                               |                          |

# Privacy

You can view privacy and data collection details for the Official Bot [here](PRIVACY.md).

# Localization

AutoMuteUs now uses [CrowdIn](https://crowdin.com/) for Localization and translations (thanks @MatadorProBr)!

Help us translate the bot here:

[![Crowdin](https://badges.crowdin.net/e/5eb1365b5fd16082e63cc54c33736adc/localized.svg)](https://automuteus.crowdin.com/automuteus)

To prepare any new strings for translation, first install goi18n v2.1.1 using the following command:
```
go install -v github.com/nicksnyder/go-i18n/v2/goi18n@v2.1.1
```

Then run the following command anytime new strings or translations are added:

```
goi18n extract -outdir locales
```

# Self-Hosting

Self-hosting requires robust knowledge and troubleshooting capability for Docker/Docker-compose, unRAID, Heroku, and/or any other networking and routing config specific to your hosting solution.

As such, **we recommend that the majority of users take advantage of our Verified bot**. The link to invite our bot can
be found here:

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
