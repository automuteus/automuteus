# User API authorization: incremental implementation

Implemented: a pure, tested policy in `authorization.go`. Members may read game
state and settings in their verified guild; owners and Discord Administrators
may edit settings. Unknown actions, missing identities, nonmembers, and mismatched
guilds are denied. Manage Server alone is intentionally insufficient.

## Implemented HTTP authentication

The existing GET endpoints `/guild/settings`, `/guild/premium`, `/game/state`,
and `/game/roomcode` now accept `Authorization: Bearer <Discord access token>`.
All require `guildID`; game endpoints also require `connectCode`. Membership is
required to read premium status; purchasing premium remains a separate policy.
No write endpoint has been added.

Go calls Discord `/users/@me` and `/users/@me/guilds` with the supplied token,
handles guild pagination, and parses permission strings as integers. Tokens from
any Discord OAuth application are intentionally accepted, supporting external
clients. The required scopes are `identify guilds`; no bot token, client secret,
or Discord gateway session is needed in the API process.

Every request revalidates with Discord: there is no authorization cache or token
persistence. Responses use `Cache-Control: no-store`. Discord 401 maps to 401,
missing scope to 403, nonmembership to 403, and rate limits/outages/malformed
upstream responses to 503. A Discord 429 with a valid cooldown of up to five
seconds is retried once after waiting for Retry-After (or JSON retry_after),
within the request deadline. This handles the guild picker immediately preceding
Go's membership check. Repeated limits and longer cooldowns still fail closed;
there is no stale-authorization fallback.
HTTP requests have timeouts and production requests do not follow redirects.
This favors revocation checks over throughput; high-frequency polling should
wait for a bounded cache/rate-limit design rather than retry aggressively.

Explicit, non-default `API_ADMIN_PASS` Basic Auth retains legacy platform access
on these routes. The default `automuteus` password no longer works on game/guild
routes. `/admin/*` remains Basic Auth only with its existing notice-write guards.
Never give the UI proxy a platform password to bypass per-user checks.

Bearer game-state responses use `MemberGameState`, an explicit allowlist of
status, voice channel, phase, region, map, and player names/colors/aliveness.
They omit capture connect codes, raw user data, message state, and room codes.
Bearer room-code reads verify the stored game's guild and connect code and then
return `{ "roomCode": "..." }` from that same snapshot. They cannot use the
global connect-code lookup or retrieve stale codes after a game disappears.
Basic Auth retains the legacy game and room-code responses. Membership grants
these reads across the guild, without channel-level permission filtering.

## Next.js integration

The sibling web app now exposes matching same-origin GET routes under `/api`:
`/api/guild/settings`, `/api/guild/premium`, `/api/game/state`, and
`/api/game/roomcode`. They forward the required guildID/connectCode parameters
and a Discord access token to these Go endpoints using `AUTOMUTEUS_API_URL`.

The browser authenticates to Next.js via its session cookie. `getServerSession`
runs the token-refresh callback and persists rotated credentials in the encrypted
cookie. The access token is captured only in a request-local server callback;
neither Discord token is exposed in the public session. Browser-supplied
Authorization headers and extra query parameters are not forwarded. No CORS
support is needed for this server-to-server flow.

The web `/api/guilds` route still calls Discord directly because this API has no
guild-list endpoint. It uses the same refresh-aware helper and preserves all
member guilds for the premium picker. The web `/settings` page now consumes the
settings route with a server selector and read-only setting groups.
See the sibling web README for usage, status handling, and refresh limitations.

Example Go API request (from the Next.js server):

```http
GET /guild/settings?guildID=123456789012345678
Authorization: Bearer <Discord access token>
```

Discord OAuth scopes permit calls to Discord. AutoMuteUs action constants are
server-side policy and are not OAuth scopes to request from Discord.

## Remaining work

- Add game and premium dashboard views; coordinate simultaneous token refreshes
  across replicas if needed.
- Add validated settings writes with concurrency protection and actor auditing;
  protect cookie-authenticated Next.js writes against CSRF.
- Consider bounded authorization caching and Discord rate-limit coordination for
  polling workloads. Do not silently retain access after verification fails.
- Add game discovery so the UI can find authorized games without knowing capture
  connect codes; current endpoints retain their existing lookup parameters.

Tests use fake Discord HTTP responses and fake stores, covering route denial,
revocation, pagination, precision, upstream failures, and game projection.

Operator delegation is deferred. It would compare current Discord member roles
(`guilds.members.read`) with stored `PermissionRoleIDs`. Editing authorization
lists should remain owner/Administrator-only. Do not copy the slash-command
fallback that grants everyone admin when both configured permission lists are
empty.

Premium purchases are a separate capability: an authenticated purchaser may
sponsor any guild without gaining membership or settings access. Subscription
management requires purchaser ownership, and entitlement activation requires
verified payment confirmation. Neither is implemented by this policy.

References: https://docs.discord.com/developers/topics/oauth2 and
https://docs.discord.com/developers/resources/user.
