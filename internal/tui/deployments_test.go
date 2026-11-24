package tui

import (
	"reflect"
	"testing"

	"github.com/tituscarl/kwatch/internal/k8s"
)

const mi = 1024 * 1024

func statsFixture() ([]k8s.DeploymentInfo, []k8s.PodInfo, map[string]k8s.PodMetrics) {
	deps := []k8s.DeploymentInfo{
		{Name: "a", Namespace: "ns1"},
		{Name: "b", Namespace: "ns1"},
		{Name: "c", Namespace: "ns1"}, // no pods
		{Name: "a", Namespace: "ns2"}, // same name, different namespace
	}
	pods := []k8s.PodInfo{
		{Name: "a-1", Namespace: "ns1", Deployment: "a", Resources: k8s.PodResources{MemLimitBytes: 100 * mi}},
		{Name: "a-2", Namespace: "ns1", Deployment: "a", Resources: k8s.PodResources{MemLimitBytes: 100 * mi}},
		{Name: "b-1", Namespace: "ns1", Deployment: "b"},
		{Name: "a-9", Namespace: "ns2", Deployment: "a", Resources: k8s.PodResources{MemLimitBytes: 100 * mi}},
		{Name: "bare", Namespace: "ns1"}, // no owning deployment
	}
	metrics := map[string]k8s.PodMetrics{
		"ns1/a-1": {MemoryBytes: 50 * mi},
		"ns1/b-1": {MemoryBytes: 30 * mi},
		"ns2/a-9": {MemoryBytes: 10 * mi},
	}
	return deps, pods, metrics
}

func TestUpdateResourceStats(t *testing.T) {
	deps, pods, metrics := statsFixture()

	d := NewDeploymentsModel(false)
	d.UpdateDeployments(deps)
	d.UpdateResourceStats(pods, metrics, true)

	want := map[string]DeploymentResourceStats{
		"ns1/a": {MemUsage: "50Mi", MemLimit: "200Mi"},
		"ns1/b": {MemUsage: "30Mi"}, // usage without a limit still shows
		"ns2/a": {MemUsage: "10Mi", MemLimit: "100Mi"},
		// ns1/c has neither usage nor limit and gets no entry
	}
	if !reflect.DeepEqual(d.resourceStats, want) {
		t.Errorf("stats mismatch:\n got %v\nwant %v", d.resourceStats, want)
	}
}

// MEM shows the spinner until metrics land; MEM LIM only needs pods. With
// metrics unavailable (nil map), MEM ends up blank instead of spinning forever.
func TestDeploymentMemColumnsWhileLoading(t *testing.T) {
	deps, pods, _ := statsFixture()

	d := NewDeploymentsModel(false)
	d.UpdateDeployments(deps)
	cols := d.columns()
	memAt := func() (usage, limit string) {
		v := d.rowValues(cols, deps[0]) // ns1/a: NAME READY UP AVAIL MEM MEMLIM ...
		return v[4], v[5]
	}

	if u, l := memAt(); u != spinnerFrame || l != spinnerFrame {
		t.Errorf("before pods: got %q/%q, want spinners", u, l)
	}

	d.UpdateResourceStats(pods, nil, false)
	if u, l := memAt(); u != spinnerFrame || l != "200Mi" {
		t.Errorf("pods only: got %q/%q, want spinner/200Mi", u, l)
	}

	d.UpdateResourceStats(pods, nil, true)
	if u, l := memAt(); u != "" || l != "200Mi" {
		t.Errorf("metrics unavailable: got %q/%q, want \"\"/200Mi", u, l)
	}
}
