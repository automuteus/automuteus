# amongusdiscord
Discord Bot to scrape Among Us on-screen data, and automatically mute/unmute players during the course of the game!

Implementation of [AmongUsBot](https://github.com/alpharaoh/AmongUsBot) but developed in Go, and with additional features and capabilities.

[![Demo Video](https://img.youtube.com/vi/c0-H6cY9RI8/0.jpg)](https://youtu.be/c0-H6cY9RI8)

Have any questions, concerns, bug reports, or just want to chat? Join the discord at https://discord.gg/ZkqZSWF!

# Installation
## Installation Video
[![Demo Video](https://img.youtube.com/vi/vo_qcwZzNzw/0.jpg)](https://youtu.be/vo_qcwZzNzw)

If you followed all the steps in the video above, you're done with the installation and can start using the bot, or see the
Usage/Commands sections below! If you prefer text instructions over videos, follow all the instructions below instead.

## Pre-Installation Steps, Important!!!
1. Install [Tesseract OCR](https://digi.bib.uni-mannheim.de/tesseract/tesseract-ocr-w64-setup-v5.0.0-alpha.20200328.exe).
After installation, you should have a `tesseract.exe` in `C:\Program Files\Tesseract-OCR\` (this is required by the bot).
2. Create an Application and Bot account for your Discord Server (requires Admin privileges on the Server in question).
    - Follow the instructions [HERE](https://github.com/denverquane/amongusdiscord/blob/master/BOT_README.md)

Congrats, you've done the hardest part; setting up the Bot and Application within Discord!

Now follow either the `Easiest` install, or the `Install From Source`:

## Easiest:
1. [Download the latest release executable (`.exe`)](https://github.com/denverquane/amongusdiscord/releases) for this bot.
    - If you download the `update.exe` in the releases, running that program will automatically pull the latest `amongusdiscord.exe` for you in the future!
2. Make a text file in the same directory as the `amongusdiscord.exe` you just downloaded. Inside, paste the contents of [`sample.env`](https://github.com/denverquane/amongusdiscord/blob/master/sample.env) (or the values in the "Configuration" section down below)
and make sure to add the `DISCORD_BOT_TOKEN`, `DISCORD_GUILD_ID`, and `DISCORD_CHANNEL_ID` that you got from the preinstallation steps. **Save the file as `final.env`**. If you're using Notepad, make sure it saves (using "Save As") as `final.env` with "All Types", and **not** `final.env` Text type ".txt".
3. Run the executable from step 2, either by double-clicking or using `./amongusdiscord.exe` in a terminal window. The bot should now be running, and you should see a message from the Bot in the Text Channel you chose in the Pre-Installation!

## Install From Source:
1. [Install Go 1.15.2](https://golang.org/dl/go1.15.2.windows-amd64.msi), but any version of Go 1.12+ should work (currently developing with Go 1.13).
2. Clone the repository using `git clone https://github.com/denverquane/amongusdiscord`.
3. Navigate to the directory with `cd amongusdiscord`, and then build the executable using `go build -o amongusdiscord.exe main.go`.
4. Proceed to steps 2 and 3 of the `Easiest` install section above.

# Sample Usage
Assuming a bot that has just been started, you can type the following commands to make sure it's running smooth:
```
.au l 
Ensure all the players you want tracked are in the list

.au t <voice channel name> 
(Optional) Specifically denote the channel you want users muted/un-muted within. Users in other voice channels will be ignored.
```
Get Playing! You can continue to play game after game, and any users that are in your tracked voice channel should be automuted (or all users in ALL voice channels if you didn't specify a tracked channel)

# Configuration
```
FULLSCREEN = true # only fullscreen is supported for now (screen resolution is automatically detected)

# Below is the default; leave commented out unless you installed tesseract elsewhere
# TESSERACT_PATH = C:\Program Files\Tesseract-OCR\tesseract.exe

# Only change this value if you encounter an issue, and also run multiple monitors
# 0 (the default) should be the primary display in a multi-monitor setup, but you may need
# to use values 0,1,2, etc if you have issues
# MONITOR = 0

# Only change if you are experiencing capture issues. The bot should autodetect the resolution of your primary display
# X_RESOLUTION = 1920
# Y_RESOLUTION = 1080

# how many seconds before players are muted after the "Imposter" or "Crewmate" text is displayed at the start
# of the game (default is 2)
# GAME_START_DELAY = 2

# how many seconds before players are muted after the "Voting Results" text is displayed (default is 6)
# GAME_RESUME_DELAY = 6

# how many seconds before players are unmuted after the "Who is the Imposter?" text is displayed (default is 0)
# DISCUSS_START_DELAY = 0

DEBUG_LOGS = false # print the OCR output for debugging

# Replace these values with those obtained in the Preinstallation steps prior
DISCORD_BOT_TOKEN = abcdefgh 
DISCORD_GUILD_ID = 12341234
DISCORD_CHANNEL_ID = 123432
```

# Similar Projects

- [AmongUsBot](https://github.com/alpharaoh/AmongUsBot). Without their original Python program
with a lot of the OCR/Discord functionality, I never would have even thought of this idea!

- [amongcord](https://github.com/pedrofracassi) great program for tracking player status and auto mute/unmute in Among Us.
Their project works like a traditional Discord bot; very easy installation!

# Bot Commands
The Discord Bot uses the `.au` prefix for any commands

|Command| Alias | Arguments | Description | Example |
|---|---|---|---|---|
|`.au help`|`.au h`|None|Print help info and command usage||
|`.au list`|`.au l`|None|Print the currently tracked players, and their in-game status (Beta)||
|`.au dead`|`.au d`|@mentions|Mark a user as dead so they remain muted during discussions|`.au d @DiscordUser1 @DiscordUser2`|
|`.au alive`|`.au al`|@mentions|Mark a user as alive so they are unmuted during discussions|`.au al @DiscordUser1 @DiscordUser2`|
|`.au track`|`.au t`|Voice Channel Name|Tell Bot to use a single voice channel for mute/unmute, and ignore other players|`.au t Voice channel name`|
|`.au bcast`|`.au b`|roomcode and region|Broadcast the room code and region to players|`.au b abcd asia`|
|`.au add`|`.au a`|@mentions|Add players to the tracked list (muted/unmuted throughout the game)|`.au a @DiscordUser2 @DiscordUser1`|
|`.au reset`|`.au r`|None|Reset the tracked player list manually (mainly for debug)||
|`.au muteall`|`.au ma`|None|Forcibly mute ALL users (mainly for debug)||
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
