package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
)

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
