package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) GetPodMetrics(namespace string) (map[string]PodMetrics, error) {
	if !c.metricsAvail {
		return nil, nil
	}

	metricsList, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	result := make(map[string]PodMetrics)
	for _, pm := range metricsList.Items {
		var totalCPU, totalMem int64
		for _, container := range pm.Containers {
			totalCPU += container.Usage.Cpu().MilliValue()
			totalMem += container.Usage.Memory().Value()
		}

		key := pm.Namespace + "/" + pm.Name
		result[key] = PodMetrics{
			CPU:    fmt.Sprintf("%dm", totalCPU),
			Memory: formatMemory(totalMem),
		}
	}

	return result, nil
}

func formatMemory(bytes int64) string {
	const (
		ki = 1024
		mi = ki * 1024
		gi = mi * 1024
	)
	switch {
	case bytes >= gi:
		return fmt.Sprintf("%dGi", bytes/gi)
	case bytes >= mi:
		return fmt.Sprintf("%dMi", bytes/mi)
	case bytes >= ki:
		return fmt.Sprintf("%dKi", bytes/ki)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
