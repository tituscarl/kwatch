package k8s

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) ListEvents(namespace string) ([]EventInfo, error) {
	events, err := c.clientset.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	var result []EventInfo
	for _, e := range events.Items {
		lastTime := e.LastTimestamp.Time
		if lastTime.IsZero() {
			lastTime = e.CreationTimestamp.Time
		}
		age := time.Since(lastTime)
		objRef := fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name)

		result = append(result, EventInfo{
			Type:      e.Type,
			Reason:    e.Reason,
			Object:    objRef,
			Message:   e.Message,
			Age:       age,
			Count:     e.Count,
			Namespace: e.Namespace,
		})
	}

	// Sort by age ascending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Age < result[j].Age
	})

	// Limit to 100 most recent
	if len(result) > 100 {
		result = result[:100]
	}

	return result, nil
}
