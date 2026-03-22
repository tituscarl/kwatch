# kwatch

A terminal UI for monitoring Kubernetes services. Get instant visibility into pod health, deployment status, resource usage, and cluster events — without juggling `kubectl` commands.

![Go](https://img.shields.io/badge/Go-1.25-blue)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)

## Features

- **Overview dashboard** — summary cards showing pod and deployment health at a glance
- **Pods view** — status, readiness, restarts, age, CPU/memory usage, memory limits, and utilization percentage
- **Deployments view** — replica status, rollout strategy, availability
- **Events view** — recent cluster events with warning highlighting
- **Detail view** — press Enter on any pod or deployment for expanded info including per-container resource requests/limits
- **Color-coded status** — green for healthy, yellow for pending, red for failed
- **Filtering** — press `/` to filter by name or status
- **Read-only** — only uses Kubernetes List/Get API calls, never modifies your cluster

## Installation

### From source

```bash
git clone https://github.com/tituscarl/kwatch.git
cd kwatch
make build
```

The binary will be at `./bin/kwatch`.

### Go install

```bash
go install github.com/tituscarl/kwatch@latest
```

## Usage

```bash
# Watch the default namespace
kwatch

# Watch a specific namespace
kwatch -n production

# Watch all namespaces
kwatch -A

# Use a specific kubeconfig context
kwatch --context gke_myproject_us-central1_mycluster

# Custom refresh interval (default: 5 seconds)
kwatch --refresh 10
```

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `1`-`4` | Switch tabs (Overview, Pods, Deployments, Events) |
| `Tab` / `Shift+Tab` | Next / previous tab |
| `j` / `k` or `↑` / `↓` | Navigate up / down |
| `Enter` | Show detail view for selected resource |
| `Esc` | Close detail view |
| `/` | Filter current view |
| `PgUp` / `PgDn` | Page up / down |
| `q` / `Ctrl+C` | Quit |

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | kubeconfig default | Namespace to watch |
| `--all-namespaces` | `-A` | `false` | Watch all namespaces |
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |
| `--context` | | | Kubernetes context to use |
| `--refresh` | | `5` | Refresh interval in seconds |

## Requirements

- Go 1.25+ (to build from source)
- A valid kubeconfig with access to a Kubernetes cluster
- `metrics-server` installed on the cluster for CPU/memory columns (optional — columns are hidden if unavailable)

## RBAC

kwatch only needs read permissions. The minimum ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kwatch-reader
rules:
  - apiGroups: [""]
    resources: ["pods", "events"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["get", "list"]
```

## Project structure

```
cmd/root.go              CLI entry point and flags
internal/k8s/            Kubernetes data layer (read-only)
  client.go              Client initialization from kubeconfig
  pods.go                Pod listing with status derivation
  deployments.go         Deployment listing
  events.go              Event listing
  metrics.go             Pod metrics (CPU/memory usage)
  types.go               Shared data types
internal/tui/            Terminal UI (Bubble Tea)
  app.go                 Root model, tab routing, refresh loop
  overview.go            Overview tab with summary cards
  pods.go                Pods table with resource columns
  deployments.go         Deployments table
  events.go              Events list
  detail.go              Detail overlay view
  styles.go              Color scheme and styles
  keys.go                Key bindings
  header.go              Header bar
  statusbar.go           Status bar
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
