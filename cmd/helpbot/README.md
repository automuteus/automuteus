# helpbot

Answers questions in the official AutoMuteUs server's `#help` forum with Claude, from the facts in
[`knowledge.md`](knowledge.md). Design and decisions: [`HELPBOT_PLAN.md`](../../HELPBOT_PLAN.md).

- **`knowledge.md`** is facts only, written for users. Update it when user-facing behavior changes.
- **`prompt.go`** holds how the bot behaves, including when to hand off to a human.

## How it behaves

- It answers the first post of each forum thread, then follow-ups from the original poster only, up to
  `HELPBOT_MAX_REPLIES` answers per thread. It waits `HELPBOT_DEBOUNCE` first, so several quick
  messages get one answer.
- It stays silent when:
  - the thread has the "Solved" or "Needs human" tag;
  - anyone with a staff role (`HELPBOT_STAFF_ROLE_IDS`) has posted in it;
  - the poster joined less than `HELPBOT_MIN_MEMBER_AGE` ago;
  - the poster's text, title included, is under `HELPBOT_MIN_CHARS`. Attachments are never read.
- Limits: each user gets `HELPBOT_THREADS_PER_USER_PER_DAY` answered threads per UTC day, and spending
  stops at `HELPBOT_DAILY_BUDGET_USD`. Both are in memory and reset on restart. The hard cap is the
  Anthropic workspace's spend limit.
- Claude returns `{answer, needs_human, reason}`. A handoff applies "Needs human" and the bot stops
  answering that thread.
- If a request fails (API error, refusal, unusable output), the bot posts a short apology and tags
  "Needs human". So during an Anthropic outage, every new thread gets tagged this way.
- Buttons work for the poster and staff only:
  - **Solved** tags the thread and archives it.
  - **Need a human** tags it "Needs human".
- Each answer is a Discord reply to the newest poster message it read. That's how the bot tells which
  messages are answered. A message posted while an answer was being written still gets its own answer
  next.
- Messages never ping anyone: allowed mentions are always empty.
- Nothing is stored. Each event rebuilds the thread from Discord, so only one copy may run (1 replica,
  `Recreate`).

## Setup

1. Create a Discord application for the bot. It must be separate from the main AutoMuteUs bot. Under
   **Bot**, turn on the **Message Content** intent.
2. Invite it to the server with **View Channel, Send Messages in Threads, Read Message History, and
   Manage Threads** on the forum. Manage Threads is what lets it apply tags and archive. The bot checks
   these at startup and refuses to start without them.
3. Create the `#help` forum with tags named exactly **Solved** and **Needs human**. To use other
   names, set `HELPBOT_SOLVED_TAG` / `HELPBOT_NEEDS_HUMAN_TAG`.
4. Create an Anthropic Console workspace with a monthly spend limit, and an API key in it.

| Variable | Default | |
|---|---|---|
| `HELPBOT_DISCORD_TOKEN` | required | The help bot's token |
| `ANTHROPIC_API_KEY` | required | |
| `HELPBOT_GUILD_ID` | required | |
| `HELPBOT_FORUM_CHANNEL_ID` | required | |
| `HELPBOT_STAFF_ROLE_IDS` | required | Comma-separated; Moderator, Helper, Developer |
| `HELPBOT_MODEL` | `claude-sonnet-5-5` | |
| `HELPBOT_EFFORT` | `low` | |
| `HELPBOT_DAILY_BUDGET_USD` | `5` | |
| `HELPBOT_THREADS_PER_USER_PER_DAY` | `3` | |
| `HELPBOT_MAX_REPLIES` | `5` | |
| `HELPBOT_MIN_CHARS` | `20` | |
| `HELPBOT_MIN_MEMBER_AGE` | `10m` | |
| `HELPBOT_DEBOUNCE` | `5s` | |
| `HELPBOT_PORT` | `8080` | `/metrics` and `/healthz` |

Run it locally against the test guild with a separate "helpbot-dev" application:

```sh
HELPBOT_DISCORD_TOKEN=… ANTHROPIC_API_KEY=… HELPBOT_GUILD_ID=759575869231988736 \
HELPBOT_FORUM_CHANNEL_ID=… HELPBOT_STAFF_ROLE_IDS=… HELPBOT_MIN_MEMBER_AGE=0s go run ./cmd/helpbot
```

To turn it off in production: `kubectl -n automuteus-helpbot scale deploy/automuteus-helpbot --replicas=0`.

## Eval

`eval/questions.txt` holds real questions, paraphrased from the support channels. Each one has an
`# expect:` line for the reviewer. To compare models, run:

```sh
ANTHROPIC_API_KEY=… go run ./cmd/helpbot eval -models claude-sonnet-5-5,claude-haiku-5-5 -out /tmp/helpbot-eval.md
```

The eval spends real credits, roughly $1 for both models. It prints a cost table and writes the answers
side by side to the report. Cache hits should cover every request but the first per model. If they're
zero, something in the system prompt is changing between requests. Don't commit the reports.

## Metrics

| Metric | Labels |
|---|---|
| `automuteus_helpbot_answers_total` | `model`, `handoff` |
| `automuteus_helpbot_skips_total` | `reason`: tagged, staff, answered, superseded, too_short, max_replies, new_member, budget, user_limit |
| `automuteus_helpbot_handoffs_total` | `reason`: the model's reason, `button`, or `error` |
| `automuteus_helpbot_errors_total` | `stage` |
| `automuteus_helpbot_buttons_total` | `button` |
| `automuteus_helpbot_tokens_total` | `model`, `kind` |
| `automuteus_helpbot_dollars_total` | `model` |
