<p align="center">
    <img src="assets/botProfilePicture.jpg" width="196">
</p>

<p align="center">
    <a href="https://github.com/denverquane/amongusdiscord/actions?query=build" alt="Build Status">
        <img src="https://github.com/denverquane/amongusdiscord/workflows/build/badge.svg" />
    </a>
    <a href="https://github.com/denverquane/amongusdiscord/graphs/contributors" alt="Contributors">
        <img src="https://img.shields.io/github/contributors/denverquane/amongusdiscord" />
    </a>
    <a href="https://discord.gg/ZkqZSWF" alt="Contributors">
        <img src="https://img.shields.io/discord/754465589958803548?logo=discord" />
    </a>
</p>

# AmongUsAutoMute (BETA) 

Discord Bot to harness Among Us game data, and automatically mute/unmute players during the course of the game!

Works in conjunction with [amonguscapture](https://github.com/denverquane/amonguscapture)

**This program is in Beta. While we are confident about the basic functionality, there will still be issues or pecularities with how the program functions! We are actively working to resolve these issues!**

Have any questions, concerns, bug reports, or just want to chat? Join the discord at https://discord.gg/ZkqZSWF!

# Requirements

1. This program must be run on a Windows PC. The program **CANNOT** be run directly on mobile phones.
2. You need a minimum of 12 open emoji slots on your server. The bot uses player emojis to link discord users to in-game player colors; it will add them automatically, but you need at least 12 slots (25 recommended).
3. You must run the discord bot, and the capture portion (See Easiest installation below) at the same time, and on the same PC (for now).

# Installation Video (click the image):

[![Installation](https://img.youtube.com/vi/LUptOv5ohNc/0.jpg)](https://youtu.be/LUptOv5ohNc)

# Installation

If you followed all the steps in the video above, you're done with the installation and can start using the bot, or see the
Usage/Commands sections below! If you prefer text instructions over videos, follow all the instructions below instead.

## Pre-Installation Steps, Important!!!
1. Create an Application and Bot account for your Discord Server (requires Admin privileges on the Server in question).
    - Follow the instructions [HERE](https://github.com/denverquane/amongusdiscord/blob/master/BOT_README.md)

Now follow either the `Easiest` install, or the `Install From Source`:

## Easiest:
1. [Download the latest release executable (`.exe`) and `final.txt`](https://github.com/denverquane/amongusdiscord/releases) for this discord bot.
If you don't see one for the current version, then simply create a new text document called `final.txt`, and put `DISCORD_BOT_TOKEN = ` as the contents.
2. Paste the Bot Token you obtained in the pre-installation into the `final.txt` file, after the `=` sign for the `DISCORD_BOT_TOKEN`.
3. Run the executable from step 1, either by double-clicking or using `./amongusdiscord.exe` in a terminal window.
4. [Download the latest `amonguscapture.exe`](https://github.com/denverquane/amonguscapture/releases). If you are running the Discord bot remotely,
you can add a `host.txt` file in the same folder with the contents `http://<host>:<port>` to point to that instance, but this is totally optional.
5. **If Among Us is already running,** then start the capture executable you downloaded in the previous step. Otherwise, start the game and **then** start the capture.

Congrats, if you followed the instructions correctly, the bot should now be running! See the Sample Usage section below for details.

## Install From Source:
1. [Install Go 1.15.2](https://golang.org/dl/go1.15.2.windows-amd64.msi), but any version of Go 1.12+ should work.
2. Clone the repository using `git clone https://github.com/denverquane/amongusdiscord`.
3. Navigate to the directory with `cd amongusdiscord`, and then build the executable using `go build -o amongusdiscord.exe main.go`.
4. Proceed to steps 2-5 of the `Easiest` install section above.

## Docker
You can also run the discord portion using docker if you prefer, it simply needs the port `8123` exposed, and you should provide your `DISCORD_BOT_TOKEN` as an env variable.
Example:
`docker run -p 8123:8123 -e DISCORD_BOT_TOKEN=<YourTokenHere> denverquane/amongusdiscord`

## Environment Variables
- `DISCORD_BOT_TOKEN`: The Bot Token used by the bot to authenticate with Discord.
- `DISCORD_BOT_TOKEN_2`: (Optional) A second Bot Token to be used to distribute the mute/deafen requests to Discord.
If you play in larger groups of 8+ people, this is recommended to not be rate-limited (delayed) by Discord when rounds change!

### Advanced. Only configure these variables if you know what you're doing
- `EMOJI_GUILD_ID`: If your bot is a member of multiple guilds, this ID can be used to specify the single guild that it should use for emojis (no need to add the emojis to ALL servers).
- `PORT`: The port the Bot will use for incoming Socket.io communications from the capture client. Defaults to 8123.
You must specify more (comma-delimited ports) if you are running with `NUM_SHARDS` > 1. For example, with 3 shards, `PORT = 8123,8124,8125`
- `EXT_PORT`: The port to use for the capture url. Must be a valid port number, or "protocol" to not include a port in the url. Defaults to PORT.
- `SERVER_URL`: The externally-accessible URL for *this* instance of the discord bot. For example, `http://test.com`.
This is used to provide the linking URI to the capture, via the Direct Message the bot sends you when typing `.au new` (in conjunction with the PORT above).
**You must specify `http://` or `https://` accordingly as part of the URL**
- `CONFIG_PATH`: Alternate filesystem path for guild config files. Defaults to `./`
- `NUM_SHARDS`: How many total bot shard instances you'll be running in your current stack.

## Google Firestore Config (Optional)
- `GOOGLE_APPLICATION_CREDENTIALS`: Path to credentials JSON file used to access the Google Firestore API and create/update config documents. If specified, be sure to specify the following PROJECT_ID below as well.
- `FIRESTORE_PROJECT_ID`: The specific Project ID for the Firestore operations you intend this bot to use.

## Deploy to Heroku
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

The app will fail the first time you deploy since the `DISCORD_BOT_TOKEN` is not set. To fix this:

1. Create a new Config var in your Heroku app's settings with the key `DISCORD_BOT_TOKEN` and the value as your bot token obtained from the pre-installation step.
2. Restart all dynos

To connect to this deployment, create a `host.txt` file in the same folder as the  `amonguscapture.exe` file with the contents `https://<host>`, where the host is your Heroku app URL and restart `amonguscapture.exe` if its already running.

# Sample Usage
To start the bot in the current channel, type the following `.au` command in Discord:
```
.au new
# Starts a game, and allows users to react to emojis to link to their in-game players
```
Get Playing!

If you want to view command usage or see the available options, type `.au` or `.au help` in your Discord channel.

If you need to add more players to the tracking list, they can be added using the reaction emojis once back in the lobby. Or, manually using `.au link @player color`.

# Bot Commands
The Discord Bot uses the `.au` prefix for any commands

|Command| Alias | Arguments | Description | Example |
|---|---|---|---|---|
|`.au help`|`.au h`|None|Print help info and command usage||
|`.au new`|`.au n`|None|Start a new game in the current text channel. Optionally accepts the room code and region|`.au n CODE eu`|
|`.au track`|`.au t`|VC Name|Tell Bot to use a single voice channel for mute/unmute, and ignore any other channels|`.au t Test Voice`|
|`.au link`|`.au l`|@name color|Manually link a discord user to their in-game color|`.au l @Soup cyan`|
|`.au refresh`|`.au r`|None|Remake the bot's status message entirely, in case it ends up too far up in the chat.||
|`.au end`|`.au e`|None|End the game entirely, and stop tracking players. Unmutes all and resets state||
|`.au unlink`|`.au u`|@name|Manually unlink a player|`.au u @player`|
|`.au settings`|`.au s`||View and change settings for the bot, such as the command prefix or mute behavior||
|`.au force`|`.au f`|stage|Force a transition to a stage if you encounter a problem in the state|`.au f task` or `.au f d`(discuss)|

# Similar Projects

- [AmongUsBot](https://github.com/alpharaoh/AmongUsBot). Without their original Python program
with a lot of the OCR/Discord functionality, I never would have even thought of this idea! Not currently maintained

- [amongcord](https://github.com/pedrofracassi) great program for tracking player status and auto mute/unmute in Among Us.
Their project works like a traditional Discord bot; very easy installation!

# Troubleshooting

- **"Websocket 400-something: Authentication Failed" Error!**
Your `DISCORD_BOT_TOKEN` is incorrect or invalid. Make sure you copied/pasted the Bot *token*, NOT the "client secret" from the Discord Developer portal

- **"Emoji ID is not a snowflake" Error! Or the bot doesn't provide emojis as reactions on the status message!**
The discord API is agonizingly slow to upload new emojis, inform bots about the presence of new/updated emojis, and delete emojis.
The easiest answer is to **give it a while** (sometimes can take almost 30 minutes), and try again.


