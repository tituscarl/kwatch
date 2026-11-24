package tui

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/tituscarl/kwatch/internal/k8s"
)

// The spinner animates until every tab has data, then its tick chain stops.
func TestSpinnerStopsWhenLoaded(t *testing.T) {
	a := &App{spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot))}
	a.spinning = true

	if _, cmd := a.Update(a.spinner.Tick()); cmd == nil {
		t.Fatal("spinner should keep ticking while data is loading")
	}

	a.update(PodsUpdatedMsg{Pods: []k8s.PodInfo{}})
	a.update(DeploymentsUpdatedMsg{})
	a.update(EventsUpdatedMsg{})
	a.update(MetricsUpdatedMsg{}) // nil map = metrics-server not installed
	if a.isLoading() {
		t.Fatal("still loading after all fetches landed")
	}
	if _, cmd := a.Update(a.spinner.Tick()); cmd != nil || a.spinning {
		t.Fatal("spinner should stop once nothing is loading")
	}
}
