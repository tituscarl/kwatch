package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
)

type logEntry struct {
	timestamp time.Time
	pod       string
	text      string
}

// GetPodLogs fetches the last N lines of logs for a pod container (read-only).
func (c *Client) GetPodLogs(namespace, podName, container string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
		Container: container,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer stream.Close()

	bytes, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(bytes), nil
}

// FollowPodLogs streams logs in real-time, sending each line to the channel.
// It blocks until the context is cancelled. The channel is closed when done.
func (c *Client) FollowPodLogs(ctx context.Context, namespace, podName, container string, tailLines int64, ch chan<- string) error {
	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
		Container: container,
		Follow:    true,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to follow logs: %w", err)
	}
	defer stream.Close()
	defer close(ch)

	scanner := bufio.NewScanner(stream)
	// Allow lines up to 1MB
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		case ch <- scanner.Text():
		}
	}

	return scanner.Err()
}

// maxConcurrentLogStreams limits parallel K8s API calls to avoid client-side throttling.
const maxConcurrentLogStreams = 10

// GetMultiPodLogs fetches logs from multiple pods with limited concurrency, merges by timestamp.
func (c *Client) GetMultiPodLogs(namespace string, pods []PodInfo, tailLines int64) (string, error) {
	if len(pods) == 0 {
		return "", nil
	}

	// Scale per-pod lines down for large pod counts
	perPod := tailLines
	if len(pods) > 10 {
		perPod = max(10, tailLines/int64(len(pods)/5))
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var allEntries []logEntry
	sem := make(chan struct{}, maxConcurrentLogStreams)

	for _, pod := range pods {
		wg.Add(1)
		go func(p PodInfo) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			container := ""
			if len(p.Containers) > 0 {
				container = p.Containers[0].Name
			}

			opts := &corev1.PodLogOptions{
				TailLines:  &perPod,
				Container:  container,
				Timestamps: true,
			}

			req := c.clientset.CoreV1().Pods(namespace).GetLogs(p.Name, opts)
			stream, err := req.Stream(context.Background())
			if err != nil {
				return
			}
			defer stream.Close()

			bytes, err := io.ReadAll(stream)
			if err != nil {
				return
			}

			tag := shortPodName(p.Name)
			lines := strings.Split(strings.TrimRight(string(bytes), "\n"), "\n")
			var entries []logEntry
			for _, line := range lines {
				if line == "" {
					continue
				}
				ts, text := parseTimestampedLine(line)
				entries = append(entries, logEntry{
					timestamp: ts,
					pod:       tag,
					text:      text,
				})
			}

			mu.Lock()
			allEntries = append(allEntries, entries...)
			mu.Unlock()
		}(pod)
	}

	wg.Wait()

	// Sort by timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].timestamp.Before(allEntries[j].timestamp)
	})

	// Cap lines
	if len(allEntries) > 5000 {
		allEntries = allEntries[len(allEntries)-5000:]
	}

	var b strings.Builder
	for i, e := range allEntries {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[%s] %s", e.pod, e.text)
	}
	return b.String(), nil
}

// maxFollowStreams is the max concurrent follow streams (like stern's --max-log-requests).
const maxFollowStreams = 50

// FollowMultiPodLogs streams logs from multiple pods into a single channel.
// Opens all streams at once (like stern) — each is a long-lived connection, not a repeated API call.
// Caps at maxFollowStreams pods.
func (c *Client) FollowMultiPodLogs(ctx context.Context, namespace string, pods []PodInfo, tailLines int64, ch chan<- string) error {
	// Cap to avoid overwhelming the API server
	targets := pods
	if len(targets) > maxFollowStreams {
		targets = targets[:maxFollowStreams]
	}

	// Scale per-pod tail lines for initial backfill
	perPod := tailLines
	if len(targets) > 10 {
		perPod = max(5, tailLines/int64(len(targets)/5))
	}

	var wg sync.WaitGroup

	for _, pod := range targets {
		wg.Add(1)
		go func(p PodInfo) {
			defer wg.Done()

			container := ""
			if len(p.Containers) > 0 {
				container = p.Containers[0].Name
			}

			opts := &corev1.PodLogOptions{
				TailLines:  &perPod,
				Container:  container,
				Follow:     true,
				Timestamps: true,
			}

			req := c.clientset.CoreV1().Pods(namespace).GetLogs(p.Name, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				return
			}
			defer stream.Close()

			tag := shortPodName(p.Name)
			scanner := bufio.NewScanner(stream)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

			for scanner.Scan() {
				_, text := parseTimestampedLine(scanner.Text())
				line := fmt.Sprintf("[%s] %s", tag, text)
				select {
				case <-ctx.Done():
					return
				case ch <- line:
				}
			}
		}(pod)
	}

	// Wait for all streams to end, then close the channel
	wg.Wait()
	close(ch)
	return nil
}

// shortPodName extracts a compact identifier from a pod name.
func shortPodName(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) >= 2 {
		// Return last two segments (replicaset hash + pod hash)
		return strings.Join(parts[len(parts)-2:], "-")
	}
	if len(name) > 12 {
		return name[len(name)-12:]
	}
	return name
}

// parseTimestampedLine splits a K8s timestamped log line into timestamp and text.
func parseTimestampedLine(line string) (time.Time, string) {
	idx := strings.Index(line, " ")
	if idx == -1 {
		return time.Time{}, line
	}
	ts, err := time.Parse(time.RFC3339Nano, line[:idx])
	if err != nil {
		return time.Time{}, line
	}
	return ts, line[idx+1:]
}
