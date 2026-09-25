-- Payment tables written by cmd/ipn. Unlike postgres.sql these are never applied by the bot or API; apply this file
-- by hand as a role that may run DDL, then grant the IPN role what it needs (see the end of the file). Every statement
-- is safe to rerun, and nothing here alters or drops an existing table.

-- One row per payment notification, as received and before any interpretation, so a notification can always be
-- replayed or re-read later. provider says whose format body is in; event_key is the provider's own ID for the
-- notification (PayPal's ipn_track_id), so a retry of the same notification lands on the same row.
create table if not exists payment_events
(
    event_id     bigserial PRIMARY KEY,
    provider     varchar(16) NOT NULL,
    event_key    varchar(64),
    received_at  timestamptz NOT NULL DEFAULT now(),
    attempts     integer     NOT NULL DEFAULT 1,
    body         bytea       NOT NULL, -- raw request body; PayPal's is form-encoded in its own charset, not always UTF-8
    kind         varchar(64),          -- PayPal txn_type, or payment_status when there is none (refunds)
    txn_id       varchar(64),
    subscription varchar(64),          -- the provider's subscription ID, when the event belongs to one
    guild_id     numeric,
    verified     boolean,              -- NULL until the provider has confirmed the notification is genuine
    processed_at timestamptz,          -- set once the event's effects are committed
    error        text                  -- why the last attempt stopped, for events that were not processed
);
create unique index if not exists payment_events_key_index ON payment_events (provider, event_key);
create index if not exists payment_events_subscription_index ON payment_events (subscription);
create index if not exists payment_events_guild_id_index ON payment_events (guild_id);

-- A premium subscription at any provider. last_payment_at uses the same clock as guilds.tx_time_unix (unix seconds),
-- and a subscription grants its tier for premium.SubDays after it, exactly as the bot reads guilds.
create table if not exists premium_subscriptions
(
    provider        varchar(16) NOT NULL,
    external_id     varchar(64) NOT NULL,
    guild_id        numeric     NOT NULL,
    tier            smallint    NOT NULL,
    status          varchar(16) NOT NULL, -- active, cancelled (still paid up until it ends), or ended
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_payment_at integer,
    cancelled_at    timestamptz,
    ended_at        timestamptz,
    PRIMARY KEY (provider, external_id)
);
create index if not exists premium_subscriptions_guild_id_index ON premium_subscriptions (guild_id);

-- The original PayPal ledger, kept exactly as the old listener wrote it: one row per PayPal transaction ID, tx the
-- notification as JSON. Created here only for databases that never had it; an existing table is left alone.
create table if not exists transactions
(
    tx_id    varchar(64),
    tx_time  integer,
    gross    real,
    tx       jsonb,
    guild_id numeric
);

-- Grants for the listener's role (ipn_user in the official deployment). Run once as the table owner:
--   GRANT SELECT, INSERT, UPDATE ON payment_events, premium_subscriptions, transactions TO ipn_user;
--   GRANT USAGE ON SEQUENCE payment_events_event_id_seq TO ipn_user;
--   GRANT SELECT, INSERT ON guilds TO ipn_user;
--   GRANT UPDATE (premium, tx_time_unix) ON guilds TO ipn_user;
--
-- The API shows a server's subscription status on the premium page and needs only to read it. Grant its role
-- (POSTGRES_USER in the API deployment) once:
--   GRANT SELECT ON premium_subscriptions TO <api role>;
