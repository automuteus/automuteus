# amongusdiscord
Discord Bot to scrape Among Us on-screen data, and automatically mute/unmute players during the course of the game!

Implementation of [AmongUsBot](https://github.com/alpharaoh/AmongUsBot) but developed in Go, and with additional features and capabilities.

[![Demo Video](https://img.youtube.com/vi/c0-H6cY9RI8/0.jpg)](https://youtu.be/c0-H6cY9RI8)

# Motivation
I'd like to extend a huge thank you to [alpharaoh](https://github.com/alpharaoh)! Without their original Python program
with a lot of this OCR/Discord functionality, I never would have even thought of this idea; huge credit to them for the inspiration
and template for a lot of this bot's functionality.

I chose to write this program because I couldn't implement features as fast as I would like on the original repository
(both because I'm not the biggest fan of Python, but also because it's not my repo), but primarily because I saw how writing
this utility in Go would help both myself, and users of the app. Go compiles to a single binary, so with the exception of the
Tesseract OCR utility, users only need a single release executable to run the bot (outside of all the Discord bot configuration
necessary on your respective server).

# Installation
## Pre-Installation Steps, Do Not Skip!
1. Install [Tesseract OCR](https://digi.bib.uni-mannheim.de/tesseract/tesseract-ocr-w64-setup-v5.0.0-alpha.20200328.exe).
After installation, you should have a `tesseract.exe` in `C:\Program Files\Tesseract-OCR\` (this is required by the bot).
2. Create an Application and Bot account for your Discord Server (requires Admin privileges on the Server in question). 

    a. Navigate to https://discord.com/developers/applications and create New Application (top right). Name it whatever you like.

    b. Click "Bot" on the left panel, then click the button on the right to Add Bot. Scroll down to the section titled `Privileged Gateway Intents`, and toggle the option for `Server Members Intent` to ensure it is enabled, then Save Changes.

    c. Scroll up to where the Bot Icon is displayed. Change its Username to whatever you like (Such as Among Us). Optionally, you can replace the icon with one provided in this repo under the [images folder](https://github.com/denverquane/amongusdiscord/tree/master/images). **But make sure to Copy the `Token` on the right, and paste it to a safe location.** We will need it later in the installation steps; this is the `DISCORD_BOT_TOKEN` in the `sample.env` file.

    d. On the left panel, click "OAuth2", and then check the box marked `bot` under `Scopes`. Then scroll down to `Bot Permissions`, and check the boxes marked `View Channels`, `Send Messages`, and `Mute Members` (or just `Administrator`, but be very careful doing this in general...).

    e. Scroll back up to `Scopes`, and copy the URL in the field that begins with `https://discord.com/api/oauth2/authorize?`. Paste this in a new browser tab, and grant the App access to whatever server you wish it to access. Close this tab when Finalized.

    f. Last step, almost there! Now we need to get the `DISCORD_GUILD_ID` and the `DISCORD_CHANNEL_ID`. Go to https://discord.com/app, and navigate to the Server you want the Bot to communicate in (Servers are also known as Guilds). Navigate to the text channel you will want the Bot to send messages and receive commands in, and observe the URL in your browser. It will have the format `https://discord.com/channels/<DISCORD_GUILD_ID>/<DISCORD_CHANNEL_ID>`. Use these ID fields to populate the `sample.env` in the installation steps below (or paste the IDs somewhere else for now, making sure to label them appropriately). If this text channel is private or limited to certain roles, you will need to manually grant access to the Bot.

Congrats, you've done the hardest part; setting up the Bot and Application within Discord!

Now follow either the `Easiest` install, or the `Install From Source`:

## Easiest:
1. [Download the latest release executable (`.exe`)](https://github.com/denverquane/amongusdiscord/releases) for this bot.
2. Place the [`sample.env`](https://github.com/denverquane/amongusdiscord/blob/master/sample.env) from this repo into the same folder as the `amongusdiscord.exe` from step 1, 
and modify values as necessary (you should've collected the `DISCORD` values in the Pre-Installation steps above). When finished, rename the file `final.env`. You can, alternatively, copy the sample text in the "Configuration" section below, and paste into any text editor, making sure to name the file
`final.env`.
3. Run the executable from step 2, either by double-clicking or using `./amongusdiscord.exe` in a terminal window! The bot should now be running, and you should see a message from the Bot in the Text Channel you chose in the Pre-Installation!

## Install From Source:
1. [Install Go 1.15.2](https://golang.org/dl/go1.15.2.windows-amd64.msi), but any version of Go 1.12+ should work (currently developing with Go 1.13).
2. Clone the repository using `git clone https://github.com/denverquane/amongusdiscord`.
3. Navigate to the directory with `cd amongusdiscord`, and then build the executable using `go build -o amongusdiscord.exe main.go`.
4. Proceed to steps 2 and 3 of the `Easiest` install section above.

# Sample Usage
Assuming a bot that has just been started, you should type the following commands in Discord:
```
.au a @<player1> @<player2> ... 
Adds all players so they are tracked

.au l 
(Optional) Ensure all the players you want tracked are in the list

.au t <voice channel name> 
(Optional) Specifically denote the channel you want users muted/unmuted within. Users in other voice channels will be ignored.
```
Get Playing! You can continue to play game after game, and any users that enter or leave the tracked voice channel (or enter/leave ANY voice channel, if you didn't run the `.au t` command) will be muted/unmuted appropriately.

Alternatively, you could run this one command -albeit with a limitation-:
```
.au t <voice channel name>
EMPTY voice channel you intend to use.

Then have all players join that voice channel.
```
The bot is incapable of fetching the full state of the server, so it either

1. Needs to be provided a full list of all users to track, or
2. Know the voice channel to track, and record users that enter/leave that voice channel.

This is a limitation of the discordgo library I am using, but any of the 2 above approaches should work.

Theoretically, if there are 0 users on the discord server currently, then all players will be automatically picked up as they enter the server voice channels. But assuming all users are already in voice, use the commands above.

# Configuration
```
FULLSCREEN = true # only fullscreen is supported for now (screen resolution is automatically detected)

DEBUG_LOGS = false # print the OCR output for debugging

# Replace these values with those obtained in the Preinstallation steps prior
DISCORD_BOT_TOKEN = abcdefgh 
DISCORD_GUILD_ID = 12341234
DISCORD_CHANNEL_ID = 123432
```

# Bot Commands
The Discord Bot uses the `.au` prefix for any commands

|Command| Alias | Arguments | Description | Example |
|---|---|---|---|---|
|`.au list`|`.au l`|None|Print the currently tracked players, and their in-game status (Beta)||
|`.au reset`|`.au r`|None|Reset the tracked player list manually (mainly for debug)||
|`.au help`|`.au h`|None|Print help info and command usage||
|`.au add`|`.au a`|@mentions|Add players to the tracked list (muted/unmuted throughout the game)|`.au a @DiscordUser2 @DiscordUser1`|
|`.au track`|`.au t`|Voice Channel Name|Tell Bot to use a single voice channel for mute/unmute, and ignore other players|`.au t Voice channel name`|
|`.au dead`|`.au d`|@mentions|Mark a user as dead so they aren't unmuted during discussions|`.au d @DiscordUser1 @DiscordUser2`|
|`.au unmuteall`|`.au ua`|None|Forcibly unmute ALL users (mainly for debug)||
# How it Works
amongusdiscord uses Tesseract for OCR (Optical Character Recognition) to scan the Among Us game screen, and determine
if a discussion is occurring, a round is starting, etc., and if it should mute/unmute players in the Discord server automatically.

The application uses scaling values, and automatically detects the screen resolution of your main display. In theory, this should mean *any* resolution in fullscreen mode
should work, but more testing is needed to say this for certain.

Work is currently being done to also keep dead players muted, and potentially allow the the meeting caller to speak first
for some amount of time, but these are much harder problems, particularly with Tesseract's limitations, and the additional
processing required.

# Troubleshooting
## Running the executable does nothing, I don't see any window pop up!
To get more information, try running the executable from a command line window. You can type `cmd` in the Windows search bar, then use `cd C:\...` to navigate to the folder where the .exe is located. Once in that folder, try running the executable with `./amongusdiscord.exe` or `amongusdiscord.exe`, and the output should help you solve the problem.

Common issues:
* Your tesseract executable is located somewhere besides `C:\Program Files\Tesseract-OCR\tesseract.exe`
* Your `final.env` file is actually named `final.env.txt`. You can rename the file using the command line and typing `rename final.env.txt final.env`

## Optical Character Recognition (Tesseract-OCR)
To diagnose OCR problems, you can run the executable with additional arguments to see the OCR output, and `.png` files of what the program captured. There are images provided in the `images` folder of the repository that can assist with troubleshooting; fullscreen these on your primary display before running the executable for debugging.

Running `./amongusdiscord.exe discuss` will print the results of the OCR, as well as produce a `discuss.png` file that shows what was captured and processed. In the case of `discuss`, this should be tested with the `discussion`, `results`, and `voting` test pngs (these all have the same text capture region).
The same is true for `./amongusdiscord.exe ending`; this should be tested with the `crewmate`, `defeat`, and `victory` images as they all have the same capture region.

If you encounter a capture problem where the region is incorrectly offset, please file an issue on this repository and include your screen resolution, in-game resolution, and any other pertinent information (bonus points for screenshots!).
