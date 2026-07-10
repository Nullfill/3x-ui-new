# Traffic Usage Multiplier

The panel setting **Traffic Usage Multiplier** applies a coefficient from 1.0
through 10.0 to newly observed client upload and download traffic. It is off by
default. Existing `client_traffics` counters are never rewritten.

## Safety model

- Local Xray statistics are raw per-poll deltas. The coefficient is applied
  once immediately before the atomic `client_traffics` increment.
- Remote node statistics are cumulative snapshots. Existing
  `node_client_traffics` baselines derive the raw node delta first; only that
  delta is multiplied on the master.
- `traffic_multiplier_states` keeps independent state per client and source
  (`source_node_id = 0` means local). This prevents cross-node or repeated-cycle
  compounding and records both raw and billed progress.
- Enabling, disabling, or changing the factor updates state configuration at
  the save boundary. It never changes billed history.
- Client traffic reset/renew removes the relevant multiplier baseline. Client
  deletion removes it, and an email rename migrates it in the same transaction.
- A decreasing cumulative node counter is treated as a reset: no negative or
  replacement traffic is billed and the baseline is refreshed.

## Manual verification

1. Create a client with zero usage.
2. Enable the multiplier and set the factor to `2`.
3. Feed a raw delta of 100 MiB up and 500 MiB down. The billed counters should
   increase by 200 MiB up and 1000 MiB down.
4. Run another poll without traffic. Counters must not change.
5. Change the factor to `3`, then feed 100 MiB down. Billed download must grow
   by exactly 300 MiB; previous usage must remain unchanged.

## Build and test

```sh
gofmt -w internal/database/model/traffic_multiplier_state.go internal/database/db.go internal/web/entity/entity.go internal/web/service/setting.go internal/web/service/traffic_multiplier.go internal/web/service/traffic_multiplier_test.go internal/web/service/inbound_traffic.go internal/web/service/inbound_node.go internal/web/service/client_traffic.go
go test ./...
cd frontend
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```
