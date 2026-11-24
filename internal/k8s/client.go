package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	// Register the oidc/gcp/azure auth providers so kubeconfigs that rely on
	// them load instead of failing with "no Auth Provider found". Exec-based
	// plugins (gke-gcloud-auth-plugin, aws eks get-token, kubelogin) work
	// without this.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// apiTimeout bounds every one-shot List/Get so a slow or unreachable API
// server can't hang startup or leave fetch goroutines stuck forever. Log
// follow streams are long-lived and use the caller's context instead.
const apiTimeout = 10 * time.Second

type Client struct {
	clientset        kubernetes.Interface
	metricsClient    metricsv.Interface
	clusterName      string
	contextName      string
	serverURL        string
	defaultNamespace string
	metricsAvail     bool
}

// reqCtx returns a context bounded by apiTimeout. Callers must defer cancel().
func (c *Client) reqCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), apiTimeout)
}

type ClusterInfo struct {
	ClusterName string
	ContextName string
	ServerURL   string
}

// NewClient builds a read-only client for the given kubeconfig context.
// awsProfile, when set, picks the AWS CLI profile used to fetch EKS tokens.
func NewClient(kubeconfig, kubeContext, awsProfile string) (*Client, error) {
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
	if err := applyAWSProfile(restConfig, awsProfile); err != nil {
		return nil, err
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

// applyAWSProfile points the kubeconfig's exec credential plugin (aws eks
// get-token, aws-iam-authenticator) at the given AWS CLI profile. It wins
// over any profile the kubeconfig already pins, whether through an env entry
// or a --profile arg.
func applyAWSProfile(cfg *rest.Config, profile string) error {
	if profile == "" {
		return nil
	}
	if cfg.ExecProvider == nil {
		return fmt.Errorf("--aws-profile needs a kubeconfig context that authenticates via an exec plugin (e.g. from `aws eks update-kubeconfig`)")
	}
	args := cfg.ExecProvider.Args
	hasFlag := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--profile" {
			args[i+1] = profile
			hasFlag = true
		}
	}
	// The aws CLI ranks AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY above
	// AWS_PROFILE, so static keys in the shell would silently win over the
	// env var. Only the --profile flag beats them.
	if !hasFlag && filepath.Base(cfg.ExecProvider.Command) == "aws" {
		cfg.ExecProvider.Args = append(args, "--profile", profile)
	}
	// Still set the env var for other plugins (aws-iam-authenticator).
	// client-go appends these after os.Environ() and exec keeps the last
	// duplicate, so appending is enough to override.
	cfg.ExecProvider.Env = append(cfg.ExecProvider.Env, clientcmdapi.ExecEnvVar{Name: "AWS_PROFILE", Value: profile})
	return nil
}

func (c *Client) probeMetrics() bool {
	ctx, cancel := c.reqCtx()
	defer cancel()
	_, err := c.metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{Limit: 1})
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
