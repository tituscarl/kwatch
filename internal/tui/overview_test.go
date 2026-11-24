package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/tituscarl/kwatch/internal/k8s"
)

func TestOverviewIssueSeverity(t *testing.T) {
	o := NewOverviewModel(false)
	o.UpdatePods([]k8s.PodInfo{
		{Name: "fresh", Status: "Pending", Age: 10 * time.Second},
		{Name: "stuck", Status: "Pending", Age: 10 * time.Minute},
		{Name: "looping", Status: "CrashLoopBackOff", Restarts: 9},
	})
	o.UpdateDeployments([]k8s.DeploymentInfo{
		{Name: "down", Desired: 2, Available: 0},
		{Name: "degraded", Desired: 3, Available: 2},
		{Name: "fine", Desired: 1, Available: 1},
	})

	got := map[string]int{}
	for _, iss := range o.issues() {
		got[iss.name] = iss.severity
	}
	want := map[string]int{
		"stuck":    issueWarning,
		"looping":  issueCritical,
		"down":     issueCritical,
		"degraded": issueWarning,
	}
	if len(got) != len(want) {
		t.Fatalf("issues = %v, want %v", got, want)
	}
	for name, sev := range want {
		if s, ok := got[name]; !ok || s != sev {
			t.Errorf("%s: severity %d (listed=%v), want %d", name, s, ok, sev)
		}
	}

	// Critical issues sort ahead of warnings
	if o.issues()[0].severity != issueCritical {
		t.Error("first issue should be critical")
	}

	// Health card counts come from the same list
	card := o.renderHealthCard(40)
	if !strings.Contains(card, "2 critical") || !strings.Contains(card, "Warning      2") {
		t.Errorf("health card out of sync with issues:\n%s", card)
	}
}
