# The Capture Mod speaks Socket.IO v2

Galactus uses `googollee/go-socket.io` v1.4.4, which is archived and only speaks Socket.IO v2 (Engine.IO 3). The Capture Mod speaks that same v2 protocol rather than waiting on a broker upgrade: it sends a handful of events and needs nothing v3+ adds, and a hand-rolled client is the same size either way.

## Considered Options

- **Upgrade Galactus to a v4 server first** (e.g. `zishang520/socket.io` with `allowEIO3` for existing Capture Apps). Worth doing someday to leave an archived library, but it's broker work in its own right; a v2 Capture Mod keeps working after it.
- **A plain WebSocket endpoint just for Capture Mods.** Rejected: a second protocol to maintain forever.
