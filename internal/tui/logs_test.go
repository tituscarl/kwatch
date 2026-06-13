package tui

import (
	"fmt"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		line string
		want logSeverity
	}{
		{"server started, all good", sevNormal},
		{"info: handling request", sevNormal},
		{"connection ERROR while dialing", sevError},
		{"panic: runtime error", sevError},
		{"a warning was logged", sevWarn},
		{"E0323 12:00:00.000 controller failed", sevError},
		{"W0323 12:00:00.000 deprecated flag", sevWarn},
		{"F0323 12:00:00.000 fatal exit", sevError},
		{"[api-7b9-x2k] E0323 oom", sevError}, // klog prefix behind a multi-pod tag
		{"[api-7b9-x2k] just chatter", sevNormal},
	}
	for _, c := range cases {
		if got := classify(c.line); got != c.want {
			t.Errorf("classify(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

// The severity cache must stay the same length as the line buffer through
// appends and trims, or rendering would index past the end.
func TestAppendLinesSeverityStaysInSync(t *testing.T) {
	var l LogsModel
	l.atBottom = true

	l.AppendLines([]string{"one", "two error", "three"})
	if len(l.severity) != len(l.lines) {
		t.Fatalf("after append: severity=%d lines=%d", len(l.severity), len(l.lines))
	}

	// Overflow the cap so a trim happens.
	big := make([]string, maxLogLines+250)
	for i := range big {
		big[i] = fmt.Sprintf("line %d", i)
	}
	l.AppendLines(big)

	if len(l.lines) != maxLogLines {
		t.Fatalf("expected buffer capped at %d, got %d", maxLogLines, len(l.lines))
	}
	if len(l.severity) != len(l.lines) {
		t.Fatalf("after trim: severity=%d lines=%d (out of sync)", len(l.severity), len(l.lines))
	}
}

// After a trim, match indices must be re-based onto the trimmed buffer: every
// recorded index has to be in range and actually point at a matching line, and
// no match may be lost.
func TestAppendLinesTrimRebasesMatches(t *testing.T) {
	var l LogsModel
	l.atBottom = true
	l.filterTerm = "error"
	l.refilter() // compiles the (case-insensitive) regex against an empty buffer

	total := maxLogLines + 200
	batch := make([]string, total)
	for i := range batch {
		if i%10 == 0 {
			batch[i] = fmt.Sprintf("line %d error here", i)
		} else {
			batch[i] = fmt.Sprintf("line %d ok", i)
		}
	}
	l.AppendLines(batch)

	if len(l.lines) != maxLogLines {
		t.Fatalf("expected buffer capped at %d, got %d", maxLogLines, len(l.lines))
	}

	want := 0
	for _, line := range l.lines {
		if l.filterRegex.MatchString(line) {
			want++
		}
	}
	if len(l.matchLines) != want {
		t.Fatalf("matchLines=%d, want %d", len(l.matchLines), want)
	}
	for _, idx := range l.matchLines {
		if idx < 0 || idx >= len(l.lines) {
			t.Fatalf("match index %d out of range [0,%d)", idx, len(l.lines))
		}
		if !l.filterRegex.MatchString(l.lines[idx]) {
			t.Fatalf("match index %d points at non-matching line %q", idx, l.lines[idx])
		}
	}
}
