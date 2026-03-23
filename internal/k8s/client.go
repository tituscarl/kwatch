package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Client struct {
	clientset        kubernetes.Interface
	metricsClient    metricsv.Interface
	clusterName      string
	contextName      string
	serverURL        string
	defaultNamespace string
	metricsAvail     bool
}

type ClusterInfo struct {
	ClusterName string
	ContextName string
	ServerURL   string
}

func NewClient(kubeconfig, kubeContext string) (*Client, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	contextName := rawConfig.CurrentContext
	if kubeContext != "" {
		contextName = kubeContext
	}

	clusterName := ""
	serverURL := ""
	defaultNS := "default"
	if ctx, ok := rawConfig.Contexts[contextName]; ok {
		clusterName = ctx.Cluster
		if ctx.Namespace != "" {
			defaultNS = ctx.Namespace
		}
		if cluster, ok := rawConfig.Clusters[ctx.Cluster]; ok {
			serverURL = cluster.Server
		}
	}

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config: %w", err)
	}

	// Raise rate limits for multi-pod log streaming (default is 5 QPS / burst 10).
	// This is a read-only tool, so higher QPS is safe.
	restConfig.QPS = 50
	restConfig.Burst = 100

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	metricsClient, err := metricsv.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	c := &Client{
		clientset:        clientset,
		metricsClient:    metricsClient,
		clusterName:      clusterName,
		contextName:      contextName,
		serverURL:        serverURL,
		defaultNamespace: defaultNS,
	}

	// Probe metrics API availability
	c.metricsAvail = c.probeMetrics()

	return c, nil
}

func (c *Client) probeMetrics() bool {
	_, err := c.metricsClient.MetricsV1beta1().PodMetricses("").List(context.Background(), metav1.ListOptions{Limit: 1})
	return err == nil
}

func (c *Client) MetricsAvailable() bool {
	return c.metricsAvail
}

func (c *Client) DefaultNamespace() string {
	return c.defaultNamespace
}

func (c *Client) ClusterInfo() ClusterInfo {
	return ClusterInfo{
		ClusterName: c.clusterName,
		ContextName: c.contextName,
		ServerURL:   c.serverURL,
	}
}
