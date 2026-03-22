package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) ListDeployments(namespace string) ([]DeploymentInfo, error) {
	deploys, err := c.clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var result []DeploymentInfo
	for _, d := range deploys.Items {
		desired := int32(0)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		age := time.Since(d.CreationTimestamp.Time)
		strategy := string(d.Spec.Strategy.Type)

		result = append(result, DeploymentInfo{
			Name:      d.Name,
			Namespace: d.Namespace,
			Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
			UpToDate:  d.Status.UpdatedReplicas,
			Available: d.Status.AvailableReplicas,
			Desired:   desired,
			Age:       age,
			Strategy:  strategy,
		})
	}
	return result, nil
}

// ListDeploymentPods returns pods belonging to a deployment (read-only).
func (c *Client) ListDeploymentPods(namespace, deploymentName string) ([]PodInfo, error) {
	deploy, err := c.clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse selector: %w", err)
	}

	pods, err := c.clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var result []PodInfo
	for _, pod := range pods.Items {
		result = append(result, podToInfo(pod))
	}
	return result, nil
}
