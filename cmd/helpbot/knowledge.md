# AutoMuteUs knowledge

Reference for answering questions in the official AutoMuteUs Discord server. Everything here is about the **official bot** (invited from add.automute.us) unless a section says otherwise.

## What AutoMuteUs is

AutoMuteUs is a Discord bot that server-mutes and deafens Among Us players in a Discord voice channel as the game moves between phases. It knows what's happening in the game because a **Capture** reports game events to it. The bot cannot see the game by itself: **without a Capture connected, nothing gets muted.**

There are two kinds of Capture:

- **Capture App** (AmongUsCapture): a standalone Windows program that reads the game from outside. Download: https://capture.automute.us
- **Capture Mod**: a BepInEx plugin loaded inside Among Us. An officially supported alternative to the Capture App: for people who play modded, and the **only way to run a Capture on Android** (through Starlight).

Only **one person per game** needs to run a Capture: usually the person who runs `/new`. Everyone else (including other mobile and console players) just needs to be in the voice channel and linked to their in-game color.

Words to use: "Capture App", "Capture Mod", "Capture" (either one), "Connect Code" (the 8-character code), "Capture Link" (the `aucapture://` link).

## Links

- Video guide (quickstart and demo): https://youtu.be/VYx6kM1O4FM
- Invite the bot: https://add.automute.us
- Website and web dashboard: https://automute.us
- Settings: https://automute.us/settings (or `/settings` in Discord)
- Stats: https://automute.us/stats (or `/stats` in Discord)
- Premium: https://automute.us/premium (or `/premium` in Discord)
- Cancel premium: https://cancelprem.automute.us
- Capture App download: https://capture.automute.us
- Capture App installer (alternative): https://github.com/automuteus/capture-install/releases/latest/download/CaptureInstaller.bat
- Capture App releases and source: https://github.com/automuteus/amonguscapture/releases
- Bot source: https://github.com/automuteus/automuteus
- Translations (Crowdin): https://automuteus.crowdin.com/automuteus
- Privacy contact: gdpr@automute.us
- Capture service health check: https://galactus.automute.us (should load and say it's alive)

Channels in this server:

- #official-bot-status: bot outages and status.
- #updates: announcements, including "Added support for the latest updates on Steam / Epic Games / MSStore" posts when the Capture App supports a new Among Us version.
- #👉│faq: the server's FAQ. #📜│rules: server rules.
- #public-dev-chat: development and modding discussion (not support).
- Server rule: **don't ping @Moderator for support**. That role is for moderation. If someone does, gently point them to #📜│rules.

## Starting a game (`/new`)

1. Join the voice channel you want to play in. The bot tracks **the voice channel you're in when you run `/new`**. If you're not in voice, it says "Please join a voice channel before starting a match!"
2. Run `/new` in a text channel.
3. You get a private reply ("🎮 Game created!") only you can see. Its message text is the bare Capture Link, like `aucapture://…/ABCD1234`.
4. Connect a Capture:
   - **Capture App**: click "Click here to connect your capture". The Capture App opens and connects. If nothing happens: download the Capture App from https://capture.automute.us, run it once, then click the link again. Or paste the `aucapture://` link into a web browser's address bar. Or connect by hand: in the Capture App open **Manual Connection**, enter the "Server URL (manual connect only)" from the reply as Host and the 8-character Code, and press **Submit**. (The reply says "press Connect"; the button is labelled Submit.) The Server URL does nothing when opened in a browser.
   - **Capture Mod**: copy the `aucapture://` link from the top of the reply, then press **Paste capture link** in the bottom-left of the Among Us main menu or lobby. On mobile Discord, long-press the message and choose **Copy Text**.
5. The public game message in the channel changes from "❌ No capture linked! Click the link above to connect! ❌" to **Main Menu**, then **Lobby** once you're in an Among Us lobby. It shows the host, the voice channel, how many players are linked, the room code, region and map.

One bot game covers many matches in a row: after each match it goes back to the lobby. It lasts until `/end`, another `/new` in that channel, or **15 minutes with no events from the Capture**, which ends it and unmutes everyone.

Running `/new` again in a channel replaces the old game with a new one and a new Connect Code. You'll need to reconnect the Capture with the new link.

Only one Capture can hold a Connect Code. If a second Capture connects with the same code (for example, you paste the link into the Capture Mod while the Capture App is open), the newest one takes over and the older one is closed.

## Linking players

Each Discord user in the voice channel has to be linked to their in-game color so the bot knows who to mute.

- **Automatic**: the bot links people whose in-game name matches their Discord username or server nickname (ignoring case and spaces), and remembers names people were linked to before in this server (for a week).
- **Yourself**: use the "Select your in-game color" menu on the game message. Pick ❌ / unlink to leave. Anyone can link themselves.
- **Someone else**: `/link user:@them color:red` and `/unlink user:@them`. These need game-control permission (see Permissions).

`/link` errors: "No game data found for the color X" means nobody with that color is in the current lobby yet. Join the lobby first, or check the color.

Unlinked players show as "**Unlinked**" in the player list and are **never muted** (unless the premium Mute Spectators setting is on). They also aren't credited in stats.

Only users already in the tracked voice channel can be linked.

Linking can't be skipped: unlinked people are left alone on purpose (so music bots aren't touched), even if alive and dead players have the same rules. For big lobbies where people struggle to link themselves, a host or "match admin" can link everyone with `/link`. Because the bot remembers in-game names, players who keep the same in-game name usually only need linking once, even if their color changes between games.

Modded clients with extra colors: the bot supports the 18 standard colors (Red, Blue, Green, Pink, Orange, Yellow, Black, White, Purple, Brown, Cyan, Lime, Maroon, Rose, Banana, Gray, Tan, Coral). Players using a color beyond those are ignored entirely: they can't be listed, linked or muted.

## How muting works

These are Discord **server** mutes and deafens, applied by the bot.

Default rules:

| Phase | Alive players | Dead players |
|---|---|---|
| Lobby | unmuted | unmuted |
| Tasks | muted and deafened | unmuted (can talk to each other) |
| Discussion (meeting) | unmuted | muted, not deafened |

- Someone killed during tasks stays muted and deafened **until the next meeting** by default, so they can't react on voice. The free setting "unmute dead during tasks" changes this.
- Server admins can change every cell of this table, and the delays, on the dashboard.
- **Delays**: by default the bot waits a few seconds before some changes (7s going into tasks, 6s from a meeting back to the lobby, 7s from a meeting to tasks, 1s from tasks to the lobby) so the game's own animations finish first. Adjustable 0–10s in settings.
- Being in the Among Us main menu unmutes everyone.
- If a linked player leaves the tracked voice channel or moves to another one, the bot unmutes and undeafens them. When they come back, the current rules apply again.
- Only linked players are muted, so music bots and other users aren't touched.
- At the end of a match, the Capture App predicts a game-ending vote and unmutes a few seconds sooner than the Capture Mod, which waits for the real game end. Both are expected.

### Why muting is slow

Discord limits how quickly a single bot can change members' mute and deafen state in a server (roughly 10 changes per 10 seconds). With many players, a round of mutes can take several seconds. This is a Discord limit, not a bug. Lowering the delays in settings doesn't help with this: the delays only wait for the game's animations to finish, and the rate limit applies either way.

Premium Silver and Gold add priority mute bots that share the work, which makes mutes much faster. Free and Bronze servers may see a "🐢 Muting is slow in this server" note on the game message when this happens. Running two or more games in one server at the same time shares the same limit, so premium is practically required for that.

Free alternative for technical users: the Capture App's **User Bot** option. You create your own Discord bot in the Discord Developer Portal, invite it to your server with Mute Members and Deafen Members, and paste its token into the Capture App. AutoMuteUs then uses that bot to share the muting work. It's more setup than premium; an older video guide: https://www.youtube.com/watch?v=jKcEW5qpk8E

## Permissions

**What the bot needs:**

- In the text channel: View Channel, Send Messages, Manage Messages, Embed Links, Use External Emojis.
- In the voice channel: Mute Members and Deafen Members.
- It does **not** need Move Members or Manage Nicknames.

If something is missing, the bot says "I'm missing the following required permissions…" and lists them. Fix the channel or role permissions, or re-invite from https://add.automute.us. Channel-level permission overrides can take away permissions the bot's role has, so check the specific channels too.

If a mute fails mid-game, the bot posts "Error in muting or deafening @user. Does the bot have permissions to mute/deafen users in #voice?" Check that the bot has Mute Members and Deafen Members in that voice channel, including any channel or category permission overrides that deny them.

**Who can use commands:**

- **Admins**: the server owner, or anyone with Administrator or Manage Server. Only admins can use `/settings`, change settings on the dashboard, and use `/premium invites`. (Older versions had an "admin user IDs" setting, `/settings admin-user-ids`; it no longer exists, and Discord permissions decide instead.)
- **Game control** (`/new`, `/end`, `/pause`, `/link`, `/unlink`): admins, plus anyone with an "operator role" set in settings. **If no operator roles are set, everyone can use these.**
- Everything else (`/help`, `/refresh`, `/stats`, `/map`, `/info`, `/privacy`, the color select menu) is open to everyone.
- Without permission: "Sorry, you don't have the required permissions to issue that command."
- The bot doesn't respond to DMs. Commands must be run in a server text channel.

## Commands

| Command | What it does |
|---|---|
| `/help [command]` | Command list and details |
| `/new` | Start a game in this channel, tracking your voice channel |
| `/refresh` | Re-post the game message (if it scrolled too far up) |
| `/pause` | Pause/unpause automuting. Pausing unmutes everyone |
| `/end` | End the game: unmutes everyone and stops tracking |
| `/link user color` | Link someone to an in-game color |
| `/unlink user` | Unlink someone |
| `/settings` | Link to the settings dashboard (admins) |
| `/stats` | Link to this server's stats dashboard |
| `/premium info` / `/premium invites` | Premium info; invite links for priority mute bots (admins) |
| `/privacy info/showme/opt-in/opt-out` | Privacy info and controls |
| `/info` | Bot version and stats |
| `/map map_name [detailed]` | Show a map image (Skeld, Mira, Polus, dlekS, Airship, Fungle) |
| `/debug unmute [user]` | Unmute someone stuck muted. Non-admins can only use it when the user isn't in a channel with an active game |
| `/debug unmute-all` | Unmute everyone tracked by the game in this channel |
| `/debug clear user` | Clear your remembered in-game names |

Spamming commands gets "Please stop spamming commands", then the bot ignores you for 5 minutes.

## Stuck muted

If someone is still server-muted or deafened after a game:

1. Run `/debug unmute-all` in the game's text channel, or `/debug unmute user:@them`.
2. If there's no game anymore, a non-admin can run `/debug unmute` on themselves; admins can unmute anyone.
3. Anyone with Mute Members can also right-click the user in Discord and remove the server mute/deafen.

Server mute is tied to the server, not the channel, so a stuck user stays muted in every voice channel of that server until it's removed.

## Settings

`/settings` gives a link to the web dashboard (https://automute.us/settings). Sign in with Discord. The server owner and members with Administrator or Manage Server can change settings.

Free settings: language, mute/deafen rules per phase, delays, unmute dead during tasks, map image style, operator roles.

To reset all settings to defaults, use the reset option on the settings dashboard; the bot uses them from the next game. The old `/settings reset` and other `/settings <name>` subcommands no longer exist; `/settings` only gives the dashboard link. If muting behaves oddly (for example people aren't unmuted at meetings), check the voice rules on the dashboard, or reset them.

Premium-only settings: post a game summary after each match (and when to delete it), a match summary channel, auto-refreshing the game message, leaderboard options, mute spectators (treat unlinked people in the voice channel as dead players), and room code display (always / spoiler / never).

Languages: German, English, Spanish, French, Italian, Japanese, Portuguese, Russian, Chinese. Other languages (such as Hebrew) aren't available. Translations are done by the community, and anyone can contribute on Crowdin: https://automuteus.crowdin.com/automuteus

## Premium

Premium is **per Discord server** and paid monthly through PayPal at https://automute.us/premium.

| Tier | Price / month | Perks |
|---|---|---|
| Bronze | US$1.50 | Priority game access, stats and leaderboards, premium settings |
| Silver | US$3.50 | Bronze, plus 1 priority mute bot |
| Gold | US$5.50 | Silver with 3 priority mute bots, plus Gold-only support channels in this Discord |

- **Priority game access**: when the bot is very busy, free servers can be refused new games ("If I start any more games, Discord will lock me out…"). Premium servers can always start games.
- **Priority mute bots** (Silver/Gold): extra bots that mute alongside the main bot, so mutes are much faster. After buying, an admin runs `/premium invites` and invites each bot. If they aren't all invited, the game message shows "💡 Priority mute bots missing".
- After buying, all commands stay the same. There's no need to `/end` a running game or start a new one; premium takes effect within a few minutes. Check with `/premium info`. The priority mute bots work automatically once invited: they need no setup or roles beyond what `/premium invites` asks for.
- Payments usually activate within a few minutes. Don't buy again while waiting; buying again starts a second, separate subscription.
- Changing tiers: cancel the current subscription first (https://cancelprem.automute.us), then buy the new tier. Remaining days don't carry over.
- When premium expires, the server goes back to free, and the priority mute bots leave the server on their own after a while.

- PayPal is the only payment method; there's no card-only, crypto, gift card, or reseller option. If PayPal declines a payment (for example the bank's verification screen never appears), the problem is between PayPal and the bank, so contact PayPal or the bank.

**Billing problems** (paid but no premium after an hour, refunds, double charges, wrong server, moving or sharing premium with another server, cancellation trouble) are handled by the team in person; nobody outside the team can see account or payment state.

## Stats

`/stats` links to the stats dashboard. Any member can view it after signing in with Discord.

- Free: server totals and win rates, each player's summary and recent matches, match rosters.
- Premium: leaderboards, detailed player stats, and match timelines.
- Not counted: matches ended early (`/end`, timeout, platform maintenance), unlinked players, people who opted out, and **Unknown Endings** (game ends the Capture can't name, like custom wins from role mods).
- The `/premium` reply says premium gives "a full suite of player stats and leaderboards"; basic stats are free, and premium adds the leaderboards and details above.
- Resetting: any member can reset their own stats for that server on the dashboard; admins can't stop that, by design (it's a privacy feature). Admins can reset the whole server or any player.
- The old `/download` command for exporting stats was removed when the stats dashboard arrived.

## Privacy

- The bot stores your Discord user ID and the in-game names you've used, only for game stats. Data is never sold or shared.
- `/privacy showme`: see what's stored about you.
- `/privacy opt-out`: stop collection and **delete your game history in every server**. This can't be undone; opting back in doesn't restore it.
- Full policy: https://github.com/automuteus/automuteus/blob/master/PRIVACY.md. Contact: gdpr@automute.us.

## Outages and notices

- During an update, game messages show "⚠️ AutoMuteUs is being updated, so the bot may be slow to respond for a few minutes." Games keep running.
- During maintenance, games are ended, everyone is unmuted, and `/new` replies "🛑 AutoMuteUs can't start games right now." Wait for the notice to clear.
- If games end with "The AutoMuteUs capture service is restarting", run `/new` again after a minute.
- "Is the bot down?" is a frequent question. Outages are rare and usually short (a cloud provider problem, or the bot's servers in a bad state). If the bot isn't responding at all, or many people report the same problem at once, check #official-bot-status; the team handles it. There's nothing users can do to fix an outage themselves.
- "Not working" right after an Among Us update is usually not an outage; see the Capture App troubleshooting below.

## Self-hosting

Self-hosting needs solid Docker and networking knowledge, and most people should use the official bot. Instructions: https://github.com/automuteus/deploy. Detailed self-hosting troubleshooting is out of scope here; point people to that repo and its issues.

## Who can play, and on what

- Muting happens in Discord, so **anyone** in the voice channel can be muted once linked: PC, mobile, or console players.
- **At least one player per game must run a Capture**, on the same device as their game. It doesn't have to be the lobby host. That can be:
  - a **Windows PC** (Steam, Epic Games, or Microsoft Store / Xbox app) running the Capture App, or the Capture Mod on Steam; or
  - an **Android** phone or tablet running Among Us through **Starlight** (https://allofus.dev/starlight/) with the Capture Mod. This is officially supported. Starlight also works on PC.
- If nobody in the lobby can run a Capture this way, AutoMuteUs can't work.
- No Capture runs on iPhone/iPad, consoles, emulators (BlueStacks etc.), macOS, or Linux. There is no supported Linux or Mac version of the Capture App (an old Linux fork exists but is unmaintained); a Windows PC, Windows VM, or Android with Starlight is needed.
- **itch.io** copies aren't supported: the volunteer who maintains game version support doesn't have the itch version. There's no date for adding it.
- Cracked copies and beta branches of Among Us are not supported. Pirated copies may or may not work, and nobody will help fix them.
- The game itself costs money on PC; there's no free Windows version. Phone players can join a PC friend's game as long as the PC friend runs the Capture, or an Android player can run the Capture Mod through Starlight.
- Mobile players: switching apps on a phone can drop you from the Among Us lobby, so link before the game starts, or have someone use `/link`. To copy text (room code, Capture Link) in Discord mobile, long-press the message and choose **Copy Text**.
- Several games can run in one server at once: one game per text channel, each tracking its own voice channel.

## Capture App

- Windows, 64-bit only.
- **.NET questions are very common.** Capture App 6.0.0 (released October 2026) and later include everything they need, so there's **no .NET to install**. If the Capture App asks for .NET, or the installer loops trying to install it, the person has an older Capture App: download the latest from https://capture.automute.us instead. Only if they must stay on an older version (5.x or earlier), it needs the **.NET 5 Desktop Runtime x64** (not .NET Framework 3.5/4.x): https://download.visualstudio.microsoft.com/download/pr/c6a74d6b-576c-4ab0-bf55-d46d45610730/f70d2252c9f452c2eb679b8041846466/windowsdesktop-runtime-5.0.1-win-x64.exe
- Install: download the zip from https://capture.automute.us, extract it, and run `AmongUsCapture.exe`. Or use the CaptureInstaller.bat, which downloads and opens it for you.
- Run it once so Windows learns to open `aucapture://` links with it.
- It updates itself: on startup it offers to update when a newer version exists. If updating fails, download it again from https://capture.automute.us.
- When it's connected, the Discord game message updates to Main Menu / Lobby. The Capture App shows states like "Waiting for Among Us", "Waiting for Lobby", MENU, LOBBY, TASKS, DISCUSSION.
- Logs: `%APPDATA%\AmongUsCapture\logs\latest.log`, or Settings → Debug → "Open log folder".
- Its **User Bot** option takes a Discord bot token you create yourself, to make muting faster (see "Why muting is slow"). It's optional: the "AutoMuteUs" connection indicator being green is what matters. The "User Bot" indicator only turns green if you've set one up.
- There's no mobile version of the Capture App; Android players use the Capture Mod through Starlight instead.

## Capture Mod

- A BepInEx 6 (IL2CPP) plugin for the **latest Steam release** of Among Us on PC, and for **Starlight** on Android and PC. On Android, copy the `aucapture://` link in Discord mobile by long-pressing the `/new` reply and choosing **Copy Text**, then press Paste capture link in the game. Most role mods (Town of Us, The Other Roles, etc.) already ship BepInEx 6. Download `AutoMuteUsCapture.dll` from https://github.com/automuteus/capture-mod/releases.
- Install: put `AutoMuteUsCapture.dll` in the game's `BepInEx/plugins` folder.
- Connect: run `/new`, copy the `aucapture://` link, then press **Paste capture link** in the bottom-left of the Among Us main menu or lobby (it's hidden during a match). If the clipboard doesn't hold a link it says "No capture link on the clipboard. Copy the aucapture:// link from /new."
- Status line under the button: green "AutoMuteUs: connected", amber "connecting…", red "not connected", or red "another capture took over this game" (someone connected another Capture with the same code).
- The link is remembered, so restarting the game reconnects to the same bot game. After a new `/new`, paste the new link.
- It doesn't need updating for every Among Us patch, because it reads the game's own state.
- It's new: first released in October 2026.
- **Mods and the Capture App**: the Capture App only supports the official, unmodded game and generally doesn't work with mods (people often ask about Endless Host Roles / EHR, Town of Us, etc.). The Capture App also guesses when a game ends after a vote-out, and role mods' extra roles (neutral killers etc.) break that guess, which can unmute everyone too early.
- **Mods and the Capture Mod**: because it runs inside the game, it can work alongside other mods, handles revives, and reports the real game end. Its first release focuses on not breaking; mods that add lots of extra roles or unusual behavior aren't officially supported yet, so results vary. Custom wins from role mods count as Unknown Endings in stats.
- Don't run the Capture App and the Capture Mod for the same game at once; whichever connects last takes over and the other stops.

## Troubleshooting

### Clicking the capture link does nothing

1. Make sure the Capture App is downloaded from https://capture.automute.us and has been run at least once, then click the link again.
2. Paste the `aucapture://` link from the top of the `/new` reply into a web browser's address bar and allow it to open AmongUsCapture.
3. Connect by hand: Manual Connection in the Capture App, Host = the Server URL from the reply, Code = the 8-character code, press Submit.

### Capture App sees the game, but the bot says "No capture linked"

1. Make sure you used the link or code from the **latest** `/new`; each `/new` makes a new code.
2. Open https://galactus.automute.us in a browser. It should load and say it's alive. If it doesn't load, something on your network (firewall, VPN, school or work network, DNS) is blocking it, or there's an outage (check #official-bot-status).
3. If it loads but the Capture App still won't connect, run the Capture App **as Administrator**; that fixes networking/DNS problems for some people.

### Connected, but nothing happens / stuck on "Waiting for Lobby" or MENU / no players or room code

- **The most common question by far**, and it nearly always comes right after an **Among Us update** ("it was working yesterday", "Waiting for Among Us", "can't recognize Among Us"). The Capture App needs updated game data ("offsets") for each new game version. Check #updates: when support is added, a post says "Added support for the latest updates on Steam / Epic Games / MSStore… Restart your AUCapture to get updated offsets." Support usually lands within a day or a few days. No reinstall or new download is needed: once the post is up, restart the Capture App (or Settings → Debug → "Reload offsets"). The log says "No offsets found" or "Outdated version of the game" until then. Players on the Capture Mod aren't affected.
- Until support lands, the only workaround is for the player running the Capture to go back to the previous Among Us version, if their store allows it. Only that one player needs to.
- Try running the Capture App **as Administrator** (right-click → Run as administrator). This fixes detection for some people.
- Make sure both Among Us and the Capture App are up to date.
- Be in an **online lobby**; players and the room code only appear there. Local games have no room code.
- Cracked or beta copies won't work (the Capture App may warn about an unsupported version).
- If the Discord message still says "No capture linked", the Capture isn't connected to this game: reconnect with the link from the latest `/new`.

### Windows / antivirus says it's a virus

The Capture App isn't code-signed (certificates are expensive), so Windows SmartScreen and some antivirus programs flag it; Windows Defender sometimes reports "Trojan:Win32/Emali.A!cl". It's a false positive and the source is public. On the SmartScreen popup click **More info → Run anyway**. For Defender: Windows Security → Virus & threat protection → Manage settings → Exclusions → Add an exclusion, and pick `AmongUsCapture.exe`.

### Capture App won't open, crashes, or is stuck loading

1. Update to the latest version from https://capture.automute.us.
2. Reset its settings: Settings → Debug → "Reset Config", or close it and delete the folder `%APPDATA%\AmongUsCapture`.
3. Check that antivirus didn't quarantine or block it (see above).
4. It's 64-bit only; it won't run on 32-bit Windows.

### Some players aren't muted, or mutes are late

- Is the player **linked**? Unlinked players are never muted. Check the game message's player list.
- Is the player in the **tracked voice channel** (shown on the game message)? People in other channels aren't muted. To change the tracked channel, run `/new` from the right channel.
- Last few players muted several seconds late, especially with 7+ players: that's Discord's rate limit on a single bot (see "Why muting is slow"). Premium Silver/Gold priority mute bots fix this. It has nothing to do with who's the impostor.
- **Game message updates fine but nobody gets muted**: almost always permissions. The specific voice channel (or its category) often has its own permission overrides that take away Mute Members / Deafen Members from the bot, or someone changed permissions and didn't press Save. Giving the bot Administrator doesn't help if a channel override denies it. Re-inviting from https://add.automute.us gives the right server-wide permissions.
- Bot error "Error in muting or deafening…": check the bot's Mute Members / Deafen Members permissions in that voice channel.
- Everyone unmuted early after the last vote-out in a role-mod game: use the Capture Mod (see Capture Mod).
- Wrong alive/dead state: run `/refresh`; if it stays wrong, `/end` and `/new`. A kill and an emergency meeting at the same instant can confuse the game itself.

### Bot messages are blank

Turn on Discord's User Settings → Chat (Text & Images) → "Show embeds and preview website links". If it's blank for everyone, the bot is probably missing Embed Links in that channel.

### Commands don't work

- AutoMuteUs uses slash commands (type `/`). The old `.au` text commands are gone.
- If the commands don't appear at all, re-invite the bot from https://add.automute.us.
- Commands don't work in DMs.

### Game ended by itself

- 15 minutes with no events from the Capture ends the game (for example, the Capture App was closed or lost connection).
- During maintenance or a capture service restart, games are ended by the AutoMuteUs team; run `/new` again afterwards.

## Common requests that aren't possible

- Moving dead players into a separate channel: not done, because moving people costs many extra Discord requests. Dead players can already talk to each other during tasks with the default rules.
- Proximity voice chat, push-to-talk control, or letting the meeting reporter speak first: not possible with what Discord and the Capture can see.
- Two players on one Discord account: not supported.
- A game timer: not a feature.
- Support for a specific mod in the Capture App: not planned. Innersloth has no official API, and keeping up with mods isn't feasible.

Problems that aren't AutoMuteUs: "Duplicate login detected" and players getting kicked from the lobby come from Among Us itself, not the bot. Among Us crashing on launch is also usually a game or mod problem, not the Capture.

Feature requests and bug reports can go to https://github.com/automuteus/automuteus/issues (bot) or https://github.com/automuteus/amonguscapture/issues (Capture App).

## What the AutoMuteUs team handles in person

These need someone from the team; there's no self-service fix:

- Payments, premium not activating after an hour, refunds, double charges, or premium on the wrong server.
- Outages: the bot not responding for anyone, or many people with the same problem at once (see #official-bot-status).
- Bans, moderation, or reports about other users.
- Self-hosting problems beyond the basic pointers above.

Other Discord bots, unofficial forks (such as "Silence Among Us"), and general Among Us gameplay questions are out of scope.
