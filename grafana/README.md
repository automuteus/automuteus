# Grafana dashboard

`automuteus-dashboard.json` is the bot fleet dashboard in Grafana's v2 dashboard schema (Grafana 12+).

Import: Dashboards → New → Import → paste the JSON. Pick your Prometheus datasource
in the **Datasource** dropdown at the top; the **Job** dropdown is populated from
`automuteus_active_games` and defaults to `automuteus-bot`.

Every panel uses `job=~"$job"`, so the token provider (which registers the same
collectors with zero values) never dilutes the numbers.

## Counting games

`automuteus_active_games` is per process, and twin pods (`automuteus-bot-N` and
`automuteus-bot-N+10`) both subscribe to every game on their shared shards. Summing
the gauge across the fleet therefore counts every game twice. The dashboard takes
the max per shard group instead, using the last digit of the pod ordinal as the
group. If `SHARD_GROUPS` ever changes from 10, update the `label_replace` regex in
the Active Games panels.

## Rows

- **Overview** – twelve stat tiles. The second line is the "should be zero" line.
- **Games** – distinct games per shard group, raw subscriptions per pod, start/end rates, end reasons.
- **Mute / Deafen** – throughput and failures by route, failure ratio, batch
  duration percentiles + heatmap, capture-client task results, worker fallbacks.
- **Discord API** – 429s per pod, message operations by type.
- **Multi-Process Coordination** – handovers, adoptions, lease waits/losses.
- **Worker Cleanup** – cleanup loop results, backlog, oldest pending check.
- **Process Health** – CPU, RSS, goroutines, GC pause per pod. Collapsed and
  empty by default: the Alloy collector in `infra/monitoring/values.yaml` only
  keeps `automuteus_.+|up`. To populate it, widen the keep regex to

  ```
  automuteus_.+|up|go_goroutines|go_gc_duration_seconds|process_cpu_seconds_total|process_resident_memory_bytes|process_start_time_seconds
  ```

  That adds roughly nine series per pod, about a quarter more ingestion at 20 pods.

Process restarts are drawn as annotations, detected as counter resets of
`automuteus_games_started_total`.
