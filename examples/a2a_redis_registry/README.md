# A2A Redis Registry Example

This example demonstrates a distributed A2A agent registry backed by Redis,
plus consistent-hash routing across healthy agents.

## What it shows

1. `RedisRegistryStore` persists `RegistryEntry` objects in Redis.
2. Agents are discovered via `/.well-known/agent.json` and registered.
3. `ShardRouter` builds a consistent hash ring from healthy agents and routes
   keys (users, sessions, tasks) to a stable agent URL.
4. When an agent fails the health check, the ring is rebuilt and its traffic is
   redistributed to the remaining healthy agents.

## Run

### Without a real Redis

The example automatically starts an embedded `miniredis` if `REDIS_URL` is not
set:

```bash
go run ./examples/a2a_redis_registry
```

### With a real Redis

```bash
export REDIS_URL="localhost:6379"
go run ./examples/a2a_redis_registry
```

## Production Notes

- Run `registry.StartBackgroundHealthCheck(ctx, interval)` to keep the health
  status up to date in the background.
- Call `router.Refresh()` after health changes (or periodically) before routing
  requests.
- Increase `replicas` (virtual nodes) for better distribution when you have a
  small number of agents.
