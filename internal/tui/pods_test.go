package tui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/tituscarl/kwatch/internal/k8s"
)

func testPods() []k8s.PodInfo {
	var pods []k8s.PodInfo
	for i := 0; i < 50; i++ {
		pods = append(pods, k8s.PodInfo{
			Name:      fmt.Sprintf("pod-%d", i),
			Namespace: fmt.Sprintf("ns-%d", i%3),
			Status:    []string{"Running", "Pending", "CrashLoopBackOff"}[i%3],
			Age:       time.Duration(i) * time.Minute,
			Restarts:  int32(i % 6),
		})
	}
	return pods
}

// The filtered+sorted view is cached and rebuilt only when the pod list,
// filter, or sort order changes — this walks every path that must refresh it.
func TestPodsFilteredCache(t *testing.T) {
	p := NewPodsModel(false, false)
	p.SetSize(120, 30)
	p.UpdatePods(testPods())

	if len(p.filtered) != 50 {
		t.Fatalf("expected 50 filtered pods, got %d", len(p.filtered))
	}

	press := func(s string) {
		for _, r := range s {
			p.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}

	// Typing a filter narrows the cache as each rune arrives
	p.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	press("crash")
	wantMatches := 0
	for _, pod := range p.pods {
		if pod.Status == "CrashLoopBackOff" {
			wantMatches++
		}
	}
	if len(p.filtered) != wantMatches {
		t.Fatalf("filter: got %d matches, want %d", len(p.filtered), wantMatches)
	}

	// Backspace re-widens
	p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if p.filter != "cras" {
		t.Fatalf("backspace: filter = %q", p.filter)
	}
	if len(p.filtered) != wantMatches {
		t.Fatalf("backspace: got %d matches, want %d", len(p.filtered), wantMatches)
	}

	// Confirm, then cycle sort to RESTARTS (starts on NAME, s advances twice)
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	p.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	for i := 1; i < len(p.filtered); i++ {
		if p.filtered[i-1].Restarts > p.filtered[i].Restarts {
			t.Fatalf("sort: not ascending by restarts at %d", i)
		}
	}
	p.Update(tea.KeyPressMsg{Code: 'S', Text: "S", Mod: tea.ModShift})
	for i := 1; i < len(p.filtered); i++ {
		if p.filtered[i-1].Restarts < p.filtered[i].Restarts {
			t.Fatalf("sort: not descending by restarts at %d", i)
		}
	}

	if _, ok := p.SelectedPod(); !ok {
		t.Fatal("SelectedPod: expected ok")
	}

	// Esc clears the filter and restores the full list
	p.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(p.filtered) != 50 {
		t.Fatalf("esc: expected 50 pods, got %d", len(p.filtered))
	}
	_ = p.View()

	// pgdown on an empty result set must not drive the cursor negative
	p.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	press("zzzznomatch")
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if p.cursor < 0 {
		t.Fatalf("cursor went negative: %d", p.cursor)
	}
	if _, ok := p.SelectedPod(); ok {
		t.Fatal("SelectedPod: expected not ok on empty result")
	}
	_ = p.View()
}
