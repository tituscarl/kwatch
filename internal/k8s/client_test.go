package k8s

import (
	"testing"

	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestApplyAWSProfile(t *testing.T) {
	// Shape written by `aws eks update-kubeconfig --profile old`
	cfg := &rest.Config{ExecProvider: &clientcmdapi.ExecConfig{
		Command: "aws",
		Args:    []string{"eks", "get-token", "--cluster-name", "c", "--profile", "old"},
		Env:     []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "old"}},
	}}
	if err := applyAWSProfile(cfg, "new"); err != nil {
		t.Fatal(err)
	}
	if got := cfg.ExecProvider.Args[5]; got != "new" {
		t.Errorf("--profile arg = %q, want new", got)
	}
	env := cfg.ExecProvider.Env
	if last := env[len(env)-1]; last.Name != "AWS_PROFILE" || last.Value != "new" {
		t.Errorf("last env = %+v, want AWS_PROFILE=new", last)
	}

	// Kubeconfig that only pins AWS_PROFILE via env (what update-kubeconfig
	// writes): we add --profile so static AWS_ACCESS_KEY_ID in the shell
	// can't take over.
	cfg = &rest.Config{ExecProvider: &clientcmdapi.ExecConfig{
		Command: "aws",
		Args:    []string{"--region", "r", "eks", "get-token", "--cluster-name", "c"},
	}}
	if err := applyAWSProfile(cfg, "new"); err != nil {
		t.Fatal(err)
	}
	args := cfg.ExecProvider.Args
	if n := len(args); n < 2 || args[n-2] != "--profile" || args[n-1] != "new" {
		t.Errorf("args = %v, want trailing --profile new", args)
	}

	// No profile: untouched
	if err := applyAWSProfile(&rest.Config{}, ""); err != nil {
		t.Errorf("empty profile should be a no-op, got %v", err)
	}
	// Profile without an exec plugin can't be honored
	if err := applyAWSProfile(&rest.Config{}, "x"); err == nil {
		t.Error("expected error when kubeconfig has no exec plugin")
	}
}
