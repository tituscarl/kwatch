package k8s

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) GetPodMetrics(namespace string) (map[string]PodMetrics, error) {
	if !c.metricsAvail {
		return nil, nil
	}

	ctx, cancel := c.reqCtx()
	defer cancel()

	metricsList, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	result := make(map[string]PodMetrics)
	for _, pm := range metricsList.Items {
		// Sum nanocores rather than millicores so sub-millicore usage
		// survives to the display layer.
		var totalCPUNano, totalMem int64
		for _, container := range pm.Containers {
			totalCPUNano += container.Usage.Cpu().ScaledValue(resource.Nano)
			totalMem += container.Usage.Memory().Value()
		}

		key := pm.Namespace + "/" + pm.Name
		result[key] = PodMetrics{
			CPU:         formatMilliCPU(totalCPUNano),
			Memory:      formatMemory(totalMem),
			MemoryBytes: totalMem,
		}
	}

	return result, nil
}

func formatMilliCPU(nanos int64) string {
	return trimZeroDecimal(fmt.Sprintf("%.1f", float64(nanos)/1e6)) + "m"
}

func formatMemory(bytes int64) string {
	const (
		ki = 1024
		mi = ki * 1024
		gi = mi * 1024
	)
	switch {
	case bytes >= gi:
		return trimZeroDecimal(fmt.Sprintf("%.1f", float64(bytes)/gi)) + "Gi"
	case bytes >= mi:
		return trimZeroDecimal(fmt.Sprintf("%.1f", float64(bytes)/mi)) + "Mi"
	case bytes >= ki:
		return trimZeroDecimal(fmt.Sprintf("%.1f", float64(bytes)/ki)) + "Ki"
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// trimZeroDecimal drops a trailing ".0" so exact values stay compact
// ("512Mi" instead of "512.0Mi") while fractional ones keep their decimal.
func trimZeroDecimal(s string) string {
	return strings.TrimSuffix(s, ".0")
}
