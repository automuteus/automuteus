# IPN listener

`cmd/ipn` receives PayPal Instant Payment Notifications at `POST /paypal-ipn` and grants premium from them. It
replaces the standalone `automuteus/ipn` repo (image `automuteus/ipn:0.0.2`) and uses the same environment variables.
The logic lives in `internal/ipn`.

## What it does differently from the old listener

- It confirms every notification with PayPal before storing anything, and answers 200 only after the effects are
  committed. When verification or the database fails it answers 500, and PayPal retries for up to four days. The old
  listener answered 200 regardless, so a failed database write lost the payment.
- It stores every verified body untouched in `payment_events`, so notifications can be replayed. Retries of one
  notification land on the same row and are applied once.
- It tracks subscriptions in `premium_subscriptions`, keyed by PayPal `subscr_id`, from signup to cancellation to end
  of term. The old listener dropped every notification without a `txn_id`.
- It dates premium from PayPal's `payment_date` rather than from when the notification arrived.
- It creates the `guilds` row when a server that has never used the bot pays. The old `UPDATE` silently did nothing in
  that case.
- A payment it can't map to a server and a premium tier (a donation, an unknown amount, a Pending payment, a wrong
  receiver or currency) goes to the ledger only. The old listener set such servers to Free.
- A server keeps whichever premium is best. Two subscriptions resolve to the higher tier, and to the lower one when
  the higher lapses; an hourly reconcile catches that case. Paid-up premium from before this listener, and premium
  granted by hand with no expiry, are never overwritten. The listener never revokes premium: an ended subscription runs
  out through the bot's usual 31 days.
- Refunds and reversals are recorded and logged for manual review, not applied automatically.
- A live listener ignores sandbox notifications (`test_ipn=1`). The old library verified those against the sandbox,
  which could grant real premium.
- It decodes the body using the `charset` PayPal names, so accented names are stored as UTF-8. It cannot repair
  names PayPal has already replaced with `\u001a`: that happens on PayPal's side when a name (Japanese, say) has no
  windows-1252 encoding, and only the UTF-8 account setting below prevents it, for messages generated after the change.
- Logs carry event IDs, transaction IDs, server IDs and tiers only. They never contain payer names or emails.

`transactions` is still written exactly as before: the same columns and the same `tx` JSON field names, plus `SubscrID`
and `IpnTrackID`. There is now one row per PayPal transaction, updated as its status changes.

## Configuration

| Variable | |
|---|---|
| `POSTGRES_ADDR` | `host:port/db` |
| `IPN_POSTGRES_USER`, `IPN_POSTGRES_PASS` | the listener's role (`ipn_user`) |
| `IPN_EMAIL` | the PayPal account's email; notifications for other receivers grant nothing |
| `IPN_PORT` | default 3000 |
| `IPN_SANDBOX` | any value: verify against the PayPal sandbox and accept only sandbox notifications |
| `REDIS_ADDR`, `REDIS_USER`, `REDIS_PASS` | optional; the bot's Redis. With it, every premium change is announced so the API's cached stats pages rebuild at once instead of on their TTL |

The listener also serves `GET /` for liveness and `GET /ready` for readiness; `/ready` pings the database.

## Deploying

1. **Apply the schema.** As the database owner, run `storage/payments.sql`, then the `GRANT` lines at its end. Every
   statement is safe to rerun, and nothing alters an existing table. The existing `transactions` table is left as it
   is; `ipn_user` gains SELECT and UPDATE on it.
2. **Set PayPal's IPN encoding to UTF-8.** This is an account setting, not part of the IPN configuration: Account
   Settings → Website payments → Update next to "PayPal button language encoding" → More Options → Encoding: UTF-8,
   and Yes to using the same encoding for data sent from PayPal. Despite the name, that last option covers IPN
   notifications, which otherwise arrive in windows-1252. Optional: the listener decodes whatever charset the
   notification names; UTF-8 just removes a step.
3. **Switch the image.** In `infra/manifests/automuteus-ipn/automuteus-ipn.yaml`, set the image to the tag CI pushes
   (`automuteus/ipn:<tag>`) and apply it. The readiness probe already points at `/ready`. If the schema or grants are
   missing, the listener exits at startup and the old pod keeps serving.
4. **Check it.** Replay a notification from PayPal's IPN history, then query
   `SELECT event_id, kind, txn_id, processed_at, error FROM payment_events ORDER BY event_id DESC LIMIT 5`.

To roll back, deploy `automuteus/ipn:0.0.2` again. The old listener ignores the new tables.

Subscriptions that already exist show up in `premium_subscriptions` from their next renewal. Until then their
premium comes from `guilds` as before.

## Operations

- **Events that didn't apply:** `SELECT event_id, kind, txn_id, guild_id, attempts, error FROM payment_events WHERE processed_at IS NULL`.
  PayPal retries these on its own schedule. `error` holds the last failure.
- **Events that applied with a note:** `... WHERE processed_at IS NOT NULL AND error IS NOT NULL`. These include
  refunds to review, donations, and payments to the wrong receiver.
- **A server's subscriptions:** `SELECT * FROM premium_subscriptions WHERE guild_id = <id>`.
- **Tests:** `go test ./internal/ipn/` runs the parsing and verification tests. Set `TEST_POSTGRES_URL` to a disposable
  database to also replay signups, payments, cancellations, ends of term, refunds and concurrent deliveries. Those
  replays run as a role holding only the grants listed in `payments.sql`.
