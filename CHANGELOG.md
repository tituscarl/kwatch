# Changelog

## v0.1.25

### Fixed

- **Memory utilization is now accurate.** Usage and limits are compared in raw
  bytes instead of re-parsing rounded display values, so a pod at 1.99Gi of a
  2Gi limit reads 99% (not 50%). Affects the `MEM%` column and the High Memory
  Usage panel on the overview.
- **Deployment memory totals no longer double-count.** Pods are grouped by their
  owning deployment (resolved via the ReplicaSet owner reference) rather than a
  name prefix, so deployments like `api` and `api-server` stay separate.
- **API calls now time out.** Every list/get and the metrics probe is bounded by
  a 10s timeout, so a slow or unreachable cluster can't hang startup or leak
  background requests. Log follow streams remain long-lived as before.
- **`--refresh 0` no longer busy-loops.** The refresh interval is held to a
  minimum of 1 second.
