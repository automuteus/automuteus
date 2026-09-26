<p align="center">
    <a href="https://automute.us/#/" alt = "Website link"><img src="assets/AutoMuteUsBanner_cropped.png" width="800"></a>
</p>
<p align="center">
    <a href="https://github.com/automuteus/automuteus/actions?query=build" alt="Build Status">
        <img src="https://github.com/automuteus/automuteus/workflows/build/badge.svg" />
    </a>
    <a href="https://codecov.io/gh/automuteus/automuteus" alt="Code Coverage">
        <img src="https://codecov.io/gh/automuteus/automuteus/graph/badge.svg" />
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
    <a href="https://automuteus.crowdin.com/automuteus" alt="localize">
        <img alt="Localize" src="https://badges.crowdin.net/e/5eb1365b5fd16082e63cc54c33736adc/localized.svg">
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

| Command     | Description                                                                                                            | Example                  |
|-------------|------------------------------------------------------------------------------------------------------------------------|--------------------------|
| `/help`     | Print help info and command usage                                                                                      |                          |
| `/new`      | Start a new game in the current text channel                                                                           |                          |
| `/refresh`  | Remake the bot's status message entirely, in case it ends up too far up in the chat.                                   |                          |
| `/pause`    | Pause the bot, and don't let it automute anyone until unpaused.                                                        |                          |
| `/end`      | End the game entirely, and stop tracking players. Unmutes all and resets state                                         |                          |
| `/link`     | Manually link a discord user to their in-game color                                                                    | `/link @Soup cyan`       |
| `/unlink`   | Manually unlink a player                                                                                               | `/unlink @Soup`          |
| `/settings` | Get a link to the web dashboard, where the bot's settings for this server are managed                                  |                          |
| `/privacy`  | View privacy and data collection information about the bot                                                             |                          |
| `/info`     | View general info about the Bot                                                                                        |                          |
| `/map`      | View an image of an in-game map in the text channel. Provide the name of the map, and if you want the detailed version | `/map skeld true`        |
| `/stats`    | Get a link to the web dashboard, where stats for this server are shown and reset                                       |                          |
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

## Repository layout

This repository builds three service binaries:

* **AutoMuteUs** (`main.go`): the Discord bot itself. Published as `automuteus/automuteus` on Docker Hub.
* **Galactus** (`cmd/galactus`): the socket.io broker that capture clients connect to. It relays game events to
  the bot over Redis, so the bot can be upgraded or restarted without severing capture connections. Published as
  `automuteus/galactus` on Docker Hub (`Dockerfile.galactus`).
* **API** (`cmd/api`): HTTP endpoints for the website, settings, game state, and capture links.
  Published as `automuteus/api` (`Dockerfile.api`), using the same release tag and commit as the bot and Galactus.

```sh
go build .               # bot
go build ./cmd/galactus  # broker
go build ./cmd/api       # HTTP API
```

### Upgrading: standalone API

The bot no longer serves HTTP on port 5000. Run the new API image against the
same Redis and Postgres instances, and route existing API traffic to it. It needs
no Discord bot token or gateway connection. Routes and Basic Auth are unchanged.
`/bot/info` reports the API build's version/commit and the shared guild, game and
user counts. It no longer includes `shardID` or `shardCount`, since the API is not
tied to a shard; the bot's `/info` Discord command continues to describe its own shard.

Start the API, verify `/ready`, and switch the API Service/reverse proxy before
rolling out bot images that remove the embedded API. Compose users should update
the sibling `deploy` repository, stop the old bot container to release its public
port, then recreate the stack; the existing public `API_PORT` mapping now belongs
to the `api` service. Use a release that publishes
all three images. The API and bot can initialize a fresh self-hosted database in
either order; schema application is serialized and idempotent. Official mode
continues to expect an existing statistics schema.

The API uses the same guild-settings reader as the bot, including lazy migration
of old Redis records. Game state is returned directly from the shared Redis JSON.
Bot health checks and Prometheus endpoints remain on the bot.

User-facing read endpoints also accept Discord OAuth bearer tokens with guild membership checks.
See [API authorization](internal/api/AUTHORIZATION.md) for scopes, response differences, and Next.js integration.

### API environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `REDIS_ADDR` | yes | Shared Redis address, using DB 0. |
| `REDIS_USER`, `REDIS_PASS` | no | Redis credentials, when applicable. |
| `POSTGRES_ADDR`, `POSTGRES_USER`, `POSTGRES_PASS` | yes | Shared Postgres connection settings. |
| `API_PORT` | no | Executable's listening port; defaults to `5000`. Compose maps its host `API_PORT` to container `SERVICE_PORT`. |
| `API_SERVER_URL` | no | Public API URL for Swagger; defaults to `http://localhost`. Also retain this on bots for capture links. |
| `API_ADMIN_PASS` | no | Basic Auth password for user `admin`; defaults to `automuteus`. Game/guild access and raising or clearing platform notices require a non-default value. |
| `API_STATS_BUILD_TIMEOUT` | no | How long one build of a stats document (`/guild/stats`, `/guild/user`, `/guild/match`) may run, as a Go duration such as `5m`; defaults to `2m`. A request that outlasts its own deadline answers `503` with `Retry-After` while the build finishes and is cached for the retry, so this is the ceiling for the largest guilds, not a request timeout. |
| `LOG_FORMAT`, `LOG_LEVEL` | no | `text` (default) or `json`; `debug`, `info` (default), `warn`, or `error`. Shared by the bot, API, and Galactus. |
| `HOST` | no | Public Galactus URL for capture links; defaults to `http://localhost:8123`. |
| `WEB_URL` | no | Bot only: public URL of the web dashboard, which `/settings`, `/stats`, and game over summaries link to; defaults to `https://automute.us`. |
| `AUTOMUTEUS_OFFICIAL` | no | Same presence-based official mode as the bot; must match the bot deployment. |

`/live` checks the API process; `/ready` checks Redis and Postgres. Both are served
on the API port. The process drains HTTP requests on SIGTERM/SIGINT.

### Bot health endpoints

Each bot process serves probes on port `8080`. `/ready` runs one check per shard in the
process (the gateway session is connected and Discord has acknowledged a heartbeat within
the last two minutes), plus `redis` and `postgres` pings, and answers `503` with a
plain-text line per check naming any that failed. It also fails until startup completes
and from the moment the process receives SIGTERM.

On SIGTERM the process drains before closing: it stops accepting interactions and
voice events (a twin process identifying with the same shard IDs handles them instead),
releases its per-game consumer leases so a standby subscriber takes over the queue at
once, announces its running games so twins subscribe to any they had not seen, waits up to
ten seconds for handlers already in flight, then closes its sessions. Every process serving
a game's guild subscribes to it (games are announced when created and rediscovered every
minute), but only the lease holder applies its events, one burst at a time, so events are
consumed in order; a holder that dies without releasing is replaced when the ten-second
lease lapses. The lease orders consumption, not Discord: a mute request the previous holder
already issued can still complete after the new holder's, which is what the desired-state
reconcile in `VOICE_DESIRED_STATE_PLAN.md` is for. A user's command rate limit is reserved when a command is admitted and lifted again
if the response never reaches Discord, so a command dropped by a restart can be retried
without a spam warning.

Subscribers retry failed game-state reads without consuming queued events or treating
the game as deleted. Inactivity timers check for queued events and recent shared
activity under the consumer lease before ending a game. Failed checks or missing
activity data leave the subscription open for another check.

`/live` always answers `200` unless `LIVENESS_GRACE` is set to a duration (for example
`10m`), in which case it fails once `/ready` has been failing continuously for that long,
so the orchestrator restarts a process whose shards have stopped reconnecting. Keep the
grace well above Discord's own reconnect timing, and remember that during a Discord
outage every replica will reach it at once.

### Bot metrics

Each bot process serves Prometheus metrics at `/metrics` on port `2112`. Every metric
measures activity across the shards in that process, starts at zero, and resets when the
process restarts. Prometheus supplies the `job` and `instance` labels when scraping each
replica directly. Labels are fixed, small sets; guild IDs, user IDs, connect codes, and
error text stay in the logs.

Voice changes:

- `automuteus_voice_changes_total{route, outcome}`: the final outcome for each user in a
  mute/deafen batch, after fallbacks. `route` is `worker` (premium worker token),
  `capture` (the capture client's own token), or `primary` (the bot's token);
  `outcome` is `applied` or `failed`. Only the primary route can be the final failure.
- `automuteus_worker_voice_failures_total`: worker-token attempts that failed and fell
  back to another route.
- `automuteus_capture_mute_tasks_total{result}`: tasks handed to capture clients.
  `applied` was acknowledged, `throttled` was withheld by the bot to protect the client's
  rate limit, `unacked` timed out (and blacklisted the client), and `error` could not be
  sent. Games with no capture client able to mute are not counted here at all; that is a
  normal condition.
- `automuteus_mute_batch_duration_seconds`: histogram of wall time per mute/deafen batch.

Games:

- `automuteus_active_games`: games whose capture events this process is subscribed to.
- `automuteus_games_started_total`: games started with `/new`.
- `automuteus_games_ended_total{reason}`: `manual` (`/end`), `replaced` (`/new` over an
  existing game), `inactivity` (capture went quiet), `capture_shutdown`,
  `critical_notice`, or `other`.
- `automuteus_game_cleanup_failures_total{step}`: end-of-game steps that failed:
  `unmute` (players may have been left muted), `record_match` (match not marked aborted),
  or `notify` (end-of-game message not posted).

Handover between processes sharing a shard:

- `automuteus_games_adopted_total{source}`: games created elsewhere that this process
  subscribed to as a standby, by how it learned of them: `announce`, `discovery` (the
  once-a-minute scan), or `guild_create` (resubscribed on reconnect).
- `automuteus_games_handed_over_total`: games whose consumer lease this process released
  mid-burst while draining, leaving queued events for a standby.
- `automuteus_consumer_lease_waits_total`: end-of-game requests that had to wait for
  another process to finish a burst before cleaning up.
- `automuteus_consumer_lease_lost_total`: times this process found a lease it believed it
  held taken by another, meaning a renewal failed or a burst stalled past the lease TTL.
  Should stay at zero; alert on it.

Message activity, `automuteus_discord_operations_total{type}`, is a set of coarse
counters: `message_create_delete`, `message_edit`, and `rate_limited`. These reflect
existing instrumentation (message edits are counted when scheduled, for example) and are
not an exact HTTP request or successful-response count. `rate_limited` counts only
rate-limit responses reported by Discord to the primary session; the bot's own capture
throttling is reported under `automuteus_capture_mute_tasks_total{result="throttled"}`
instead. Go runtime and process metrics are also exposed by the default Prometheus
registry.

Recording metrics uses in-memory counters and makes no Redis requests. This replaces
`discord_requests_by_node_and_type` and its `nodeID` label; `SCW_NODE_ID` is no longer
used. The derived `official_request` category is removed, `invalid_request` is renamed
to `rate_limited`, and the `mute_deafen_official`, `mute_deafen_worker`, and
`mute_deafen_capture` types are replaced by `automuteus_voice_changes_total`. Old
`automuteus:requests:type:*` Redis counters are no longer read or updated. Capture clients' own Discord requests, and the bot's HTTP traffic in
general, are not yet measured. A metrics scraper and dashboards are not bundled with the
bot yet.

### Worker bot cleanup

Workers maintain a small guild-ID inventory from gateway events; cleanup does not
probe Discord membership or run from mute/deafen batches. A background sweep checks
one guild's effective premium allowance at a time, including expiry and transfers.
Excess workers enter a persistent Redis queue, with a stable token-hash order deciding
which workers to retain. Servers without matches are checked too.

`WORKER_CLEANUP_CHECK_INTERVAL` defaults to `5s` (minimum `1s`).
`WORKER_CLEANUP_LEAVE_INTERVAL` defaults to `45s` (minimum `30s`); each departure
attempt adds up to one-third of that interval as jitter, giving 45–60 seconds by
default. Both settings accept Go durations such as `30s` or `2m`. These are shared
fleet-wide budgets in Redis, not allowances per process. Failures consume the leave
budget too; cleanup does not immediately retry Discord requests or catch up in bursts
after a restart. At the default rate, 10,000 guilds take roughly 14 hours to check,
plus any time spent on errors or Discord calls; departures drain separately.

Before leaving, cleanup rechecks premium and recent game activity. Active games,
unavailable/disconnected guild inventories, recent local voice traffic, observed rate
limits, and database/Redis errors defer departures. A renewal can therefore cancel
queued cleanup. Workers with incomplete startup inventories prevent cleanup until
membership is known. Sessions that fail to open are removed from that inventory so
they cannot permanently block healthy workers. No full Discord guild cache is needed.
Free, Bronze, and voting-trial servers have no priority workers; Silver retains one,
Gold three, and self-hosted installations up to 100.

Monitor `automuteus_worker_cleanup_total{result}` (`checked`, `left`, `deferred`,
`failed`, `rate_limited`), `automuteus_worker_cleanup_pending_guilds`, and
`automuteus_worker_cleanup_oldest_check_seconds`. The gauges are each process's latest
observation of the shared queue; use `max`, not `sum`, across processes. The age tracks
check **attempts**; monitor failures as well to spot unsuccessful sweeps.

Example PromQL for cleanup progress and backlog:

```promql
sum by (result) (increase(automuteus_worker_cleanup_total[1h]))
max(automuteus_worker_cleanup_pending_guilds)
max(automuteus_worker_cleanup_oldest_check_seconds)
```

Alert on a stalled sweep when deferrals keep occurring but no checks succeed:

```promql
(sum(increase(automuteus_worker_cleanup_total{result="deferred"}[30m])) > 0)
and
(sum(increase(automuteus_worker_cleanup_total{result="checked"}[30m])) == 0)
```

Use a window longer than the configured check interval. Also alert on an increase in
`result="failed"` or `result="rate_limited"`. Failed lookups rotate to the back of the
sweep and retry on a later pass, so `oldest_check_seconds` alone cannot prove checks
are succeeding. These are query examples; no alerting service is installed by the bot.

### Platform notices

Operators can show a banner on every running game's status message, or end every
game, through `/admin/notice` (Basic Auth as `admin`, non-default `API_ADMIN_PASS`):

```sh
# warn players; games keep running; the banner stays until you DELETE the notice
curl -u admin:$API_ADMIN_PASS -X POST $API/admin/notice \
  -H 'Content-Type: application/json' \
  -d '{"severity":"warning","message":"Database maintenance in progress; expect some lag."}'

# end every running game (players are unmuted, matches recorded as aborted) and block /new until cleared
curl -u admin:$API_ADMIN_PASS -X POST $API/admin/notice \
  -H 'Content-Type: application/json' \
  -d '{"severity":"critical","message":"AutoMuteUs is going down for maintenance."}'

curl -u admin:$API_ADMIN_PASS $API/admin/notice            # show the active notice
curl -u admin:$API_ADMIN_PASS -X DELETE $API/admin/notice  # clear it
```

Galactus announces its own shutdown on SIGTERM, naming the games whose capture
clients were connected to that replica, so rolling restarts only end the games
that actually lose their capture connection; no notice is raised. Preview how the banners look
in a channel of your choice without running the bot:

```sh
go run ./cmd/embedpreview -webhook <discord webhook url> -severity warning -message "Expect some lag"
```

Regenerate Swagger documentation with the generator matching the Go dependency:

```sh
CGO_ENABLED=0 go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go -o docs --parseDependency --parseInternal
```

### Upgrading from 8.x or earlier

Guild settings are stored in Postgres. 8.x and earlier kept them in Redis, and
9.2.x is the last release that can move them. Upgrade to 9.2.x first and run
its `cmd/migrate-guild-settings` sweep before moving to 10.0 or later;
otherwise every guild falls back to the default settings. See
[storage/GUILD_SETTINGS.md](storage/GUILD_SETTINGS.md).

### Galactus environment variables

| Variable      | Required | Description                                                                   |
|---------------|----------|-------------------------------------------------------------------------------|
| `REDIS_ADDR`  | yes      | Address of the Redis instance shared with the bot.                            |
| `BROKER_PORT` | no       | Port to listen on for capture-client socket connections. Defaults to `8123`.  |
| `REDIS_USER`  | no       | Username to authenticate with Redis, if applicable.                           |
| `REDIS_PASS`  | no       | Password to authenticate with Redis, if applicable.                           |
| `DRAIN_SECONDS` | no     | Seconds to keep serving after SIGTERM, refusing new capture clients, before telling the bots to end this replica's games. Defaults to `5`. Keep well under the orchestrator's termination grace period. |
| `LOG_FORMAT`, `LOG_LEVEL` | no | Same logging switches as the bot and API. |

`/` is the liveness endpoint. `/ready` returns 503 from the moment Galactus receives
SIGTERM, so point readiness probes at it to stop routing new capture clients to a
replica that is shutting down. The container runs the binary as PID 1 (exec-form
`ENTRYPOINT`) so the signal is delivered directly; keep it that way.

### Capture mock development client

Run `go run ./cmd/capture-mock` to play simulated games through Galactus without
Among Us. Paste a connect code or capture link from `/new`, pick a scenario such as
a full round or a player being killed, and confirm at each checkpoint that the bot
muted, unmuted, and updated the status message as expected. Events can also be sent
one at a time. See [the capture mock guide](cmd/capture-mock/README.md) for usage.

### Seeding fake match history

`go run ./cmd/seed-stats` prints SQL that records a few months of made-up
games for one guild, so the stats pages have leaderboards to show
on a development stack. It prints rather than connects, so it works with the
compose stack's unpublished database ports:

```sh
go run ./cmd/seed-stats -guild <guild ID> -include <your user ID> | docker exec -i deploy-postgres-1 psql -U postgres -v ON_ERROR_STOP=1
go run ./cmd/seed-stats -guild <guild ID> -redis | docker exec -i deploy-redis-1 redis-cli   # cache the fake players' profiles
go run ./cmd/seed-stats -guild <guild ID> -clean | docker exec -i deploy-postgres-1 psql -U postgres   # remove them again
```

Seeded games use connect codes starting with `SEED`, which is all `-clean`
removes. The same `-seed` always produces the same players and outcomes; only
the timestamps follow the time of the run. Each game also gets up to `-guests`
(default 2) unlinked players, who appear only in its events and game over
report, as they would for a real lobby.

# Similar Projects

- [Imposter](https://github.com/molenzwiebel/Impostor): Similar bot that uses private Discord channels instead of mute/deafen. Also uses a dummy player joining the game and "spectating" to get game information; no capture needed (although loses the 10th player slot).

- [AmongUsBot](https://github.com/alpharaoh/AmongUsBot): Without their original Python program
  with a lot of the OCR/Discord functionality, I never would have even thought of this idea! **Not currently maintained**

- [amongcord](https://github.com/pedrofracassi/amongcord): A great program for tracking player status and auto mute/unmute in Among Us.
  Their project works like a traditional Discord bot; very easy installation!

- [Silence Among Us](https://github.com/tanndev/silence-among-us#silence-among-us): Another bot quite similar to this one, which also uses AmongUsCapture. Now in early-access with a publicly-hosted instance!
