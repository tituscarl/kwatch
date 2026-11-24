A terminal UI for monitoring Kubernetes services. Get instant visibility into pod health, deployment status, resource usage, and cluster events — without juggling `kubectl` commands.

![Go](https://img.shields.io/github/go-mod/go-version/tituscarl/kwatch)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)

> **Note:** kwatch is tested on **Google Kubernetes Engine (GKE)**. **Amazon EKS** works through the AWS CLI (v2 on your `PATH`); see [EKS](#eks).

## Features

- **Overview dashboard** — summary cards showing pod and deployment health at a glance
- **Pods view** — status, readiness, restarts, age, CPU/memory usage, memory limits, and utilization percentage
- **Deployments view** — replica status, rollout strategy, availability
- **Events view** — recent cluster events with warning highlighting
- **Log viewer** — view pod logs with snapshot or real-time follow mode, with error/warning line highlighting
- **Detail view** — press Enter on any pod or deployment for expanded info including per-container resource requests/limits
- **OOMKilled detection** — red alert banner on overview, `OOM!` tag on affected pods, even after restart
- **Color-coded status** — green for healthy, yellow for pending, red for failed
- **Memory utilization** — `MEM%` column shows usage vs limit, color-coded (green <70%, yellow 70-90%, red >90%)
- **Filtering** — press `/` to filter by name or status, navigate filtered results with arrow keys
- **Themes** — 4 built-in color themes
- **Read-only** — only uses Kubernetes List/Get API calls, never modifies your cluster

## Installation

### Go install (recommended)

```bash
go install github.com/tituscarl/kwatch@latest

# or specific version:
go install github.com/tituscarl/kwatch@v0.1.1
```

### From source

```bash
git clone https://github.com/tituscarl/kwatch.git
cd kwatch
make build
```

The binary will be at `./bin/kwatch`.

## Usage

```bash
# help
kwatch -h

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

# Set color theme
kwatch --theme everforest
```

## Keyboard shortcuts

### General

| Key | Action |
|-----|--------|
| `1`-`4` | Switch tabs (Overview, Pods, Deployments, Events) |
| `Tab` / `Shift+Tab` | Next / previous tab |
| `j` / `k` or `↑` / `↓` | Navigate up / down |
| `Enter` | Show detail view for selected resource |
| `l` | View logs for selected pod or deployment |
| `/` | Filter current view |
| `Esc` | Close current overlay (detail, logs, filter) |
| `q` / `Ctrl+C` | Quit |

### Log viewer

| Key | Action |
|-----|--------|
| `f` | Toggle follow mode (real-time tailing) |
| `j` / `k` or `↑` / `↓` | Scroll up / down |
| `G` | Jump to bottom |
| `g` | Jump to top |
| `PgUp` / `PgDn` | Page up / down |
| `Enter` | View full line |
| `/` | Grep logs (`n`/`N` next/prev match, `i` toggle case) |
| `c` | Copy the highlighted line to the clipboard |
| `t` | Cycle time window (off → 1h → 6h → 24h) |
| Mouse drag | Select text and copy it on release |
| `Esc` | Close log viewer |

### Log modes

- **SNAPSHOT** — fetches last 200 lines, refreshes every tick interval
- **FOLLOWING** — real-time log streaming, new lines appear instantly

With a time window active (`t`), snapshot fetches pull everything logged in the
past 1/6/24 hours (up to the 5000-line buffer) instead of the last 200 lines.

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | kubeconfig default | Namespace to watch |
| `--all-namespaces` | `-A` | `false` | Watch all namespaces |
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |
| `--context` | | | Kubernetes context to use |
| `--refresh` | | `5` | Refresh interval in seconds |
| `--aws-profile` | | | AWS CLI profile for EKS authentication |
| `--theme` | | `github-dark` | Color theme |

## EKS

Add the cluster to your kubeconfig with the AWS CLI, then point kwatch at it:

```bash
aws eks update-kubeconfig --name my-cluster --region us-east-1 --profile my-profile
kwatch --context arn:aws:eks:us-east-1:123456789012:cluster/my-cluster
```

`--aws-profile` swaps the profile without regenerating the kubeconfig
(`AWS_PROFILE=other kwatch` works too, unless the kubeconfig pins a profile).
For SSO profiles, run `aws sso login --profile my-profile` first.

## Themes

| Theme | Description |
|-------|-------------|
| `dracula` | Dracula — purple accent, vibrant colors |
| `github-dark` | GitHub Dark — blue accent (default) |
| `everforest` | Everforest — soft green, easy on the eyes |

## Requirements

- Go 1.27+ (to build from source)
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
    resources: ["pods", "pods/log", "events"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["get", "list"]
```

## Built with

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [client-go](https://github.com/kubernetes/client-go) — Kubernetes Go client

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
