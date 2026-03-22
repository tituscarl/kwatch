package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) ListPods(namespace string) ([]PodInfo, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var result []PodInfo
	for _, pod := range pods.Items {
		result = append(result, podToInfo(pod))
	}
	return result, nil
}

func podToInfo(pod corev1.Pod) PodInfo {
	status := derivePodStatus(pod)
	ready := podReadiness(pod)
	restarts := podRestarts(pod)
	age := time.Since(pod.CreationTimestamp.Time)

	// Build a map of spec containers for resource requests/limits
	specMap := make(map[string]corev1.Container)
	for _, c := range pod.Spec.Containers {
		specMap[c.Name] = c
	}

	var containers []ContainerInfo
	var totalCPUReq, totalCPULim, totalMemReq, totalMemLim int64

	for _, cs := range pod.Status.ContainerStatuses {
		ci := ContainerInfo{
			Name:     cs.Name,
			Ready:    cs.Ready,
			Restarts: cs.RestartCount,
			Image:    cs.Image,
		}
		if cs.State.Running != nil {
			ci.State = "Running"
		} else if cs.State.Waiting != nil {
			ci.State = cs.State.Waiting.Reason
		} else if cs.State.Terminated != nil {
			ci.State = cs.State.Terminated.Reason
		}

		// Get resource requests/limits from spec
		if spec, ok := specMap[cs.Name]; ok {
			if req := spec.Resources.Requests; req != nil {
				if cpu := req.Cpu(); cpu != nil {
					ci.CPUReq = fmt.Sprintf("%dm", cpu.MilliValue())
					totalCPUReq += cpu.MilliValue()
				}
				if mem := req.Memory(); mem != nil {
					ci.MemReq = formatMemory(mem.Value())
					totalMemReq += mem.Value()
				}
			}
			if lim := spec.Resources.Limits; lim != nil {
				if cpu := lim.Cpu(); cpu != nil {
					ci.CPULim = fmt.Sprintf("%dm", cpu.MilliValue())
					totalCPULim += cpu.MilliValue()
				}
				if mem := lim.Memory(); mem != nil {
					ci.MemLim = formatMemory(mem.Value())
					totalMemLim += mem.Value()
				}
			}
		}

		containers = append(containers, ci)
	}

	resources := PodResources{}
	if totalCPUReq > 0 {
		resources.CPUReq = fmt.Sprintf("%dm", totalCPUReq)
	}
	if totalCPULim > 0 {
		resources.CPULim = fmt.Sprintf("%dm", totalCPULim)
	}
	if totalMemReq > 0 {
		resources.MemReq = formatMemory(totalMemReq)
	}
	if totalMemLim > 0 {
		resources.MemLim = formatMemory(totalMemLim)
	}

	return PodInfo{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Status:     status,
		Ready:      ready,
		Restarts:   restarts,
		Age:        age,
		Node:       pod.Spec.NodeName,
		Resources:  resources,
		Containers: containers,
	}
}

func derivePodStatus(pod corev1.Pod) string {
	// Check for terminating
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	// Check init container statuses
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return "Init:" + cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return "Init:Error"
		}
	}

	// Check container statuses for waiting reasons
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
	}

	// Check container statuses for terminated reasons
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			if cs.State.Terminated.Reason != "" {
				return cs.State.Terminated.Reason
			}
			if cs.State.Terminated.ExitCode != 0 {
				return "Error"
			}
			return "Completed"
		}
	}

	// Fall back to pod phase
	return string(pod.Status.Phase)
}

func podReadiness(pod corev1.Pod) string {
	total := len(pod.Spec.Containers)
	ready := 0
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func podRestarts(pod corev1.Pod) int32 {
	var total int32
	for _, cs := range pod.Status.ContainerStatuses {
		total += cs.RestartCount
	}
	return total
}
