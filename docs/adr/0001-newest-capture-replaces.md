# The newest Capture replaces the one holding a Connect Code

Two Captures reporting the same game make the bot flap: the Capture App predicts a vote-out ends the game and unmutes everyone, then the Capture Mod reports tasks and they're muted again. So Galactus lets only one Capture hold a Connect Code, and a newly connecting Capture replaces the holder by sending it `killself`, which every Capture App since 2.4.4 obeys by exiting.

## Considered Options

Rejecting the newcomer was the obvious alternative, but Galactus takes up to ~80s (20s ping interval + 60s timeout) to notice a connection that dropped without closing. Rejection would lock a Capture App out of its own game after every network blip; replacement makes the reconnect instant, because a dead connection can't object.

## Consequences

- Pasting a Capture Link into the Capture Mod closes a running Capture App for that code.
- A replaced Capture must stop retrying, or two Captures replace each other forever.
- Galactus doesn't send a disconnect for a Capture it replaced, so a late disconnect can't mark the newcomer's game unlinked.
- Galactus runs several replicas, so who holds a Connect Code lives in Redis, not in one broker's memory.
