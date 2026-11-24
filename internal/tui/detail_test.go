package tui

import (
	"strings"
	"testing"

	"github.com/tituscarl/kwatch/internal/k8s"
)

// Deployment detail shows only its own pods, and surfaces both broken pods
// and pods near their memory limit under Problems.
func TestDeploymentDetailPodsAndProblems(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "api-1", Namespace: "default", Deployment: "api", Status: "CrashLoopBackOff", Ready: "0/1"},
		{Name: "api-2", Namespace: "default", Deployment: "api", Status: "Running", Ready: "1/1",
			Resources: k8s.PodResources{MemLim: "100Mi", MemLimitBytes: 100}},
		{Name: "other-1", Namespace: "default", Deployment: "other", Status: "Running", Ready: "1/1"},
	}
	metrics := map[string]k8s.PodMetrics{"default/api-2": {Memory: "95Mi", MemoryBytes: 95}}

	d := NewDetailModel()
	d.ShowDeployment(k8s.DeploymentInfo{Name: "api", Namespace: "default", Ready: "1/2", Desired: 2, Available: 1}, pods, metrics)

	for _, want := range []string{"Pods (2):", "api-1", "CrashLoopBackOff", "Under-replicated", "High memory", "95% of 100Mi"} {
		if !strings.Contains(d.content, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(d.content, "other-1") {
		t.Error("listed a pod from another deployment")
	}
}
