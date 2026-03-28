package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tituscarl/kwatch/internal/k8s"
	"github.com/tituscarl/kwatch/internal/tui"
)

var Version = "dev"

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
}

var (
	kubeconfig    string
	kubeContext   string
	namespace     string
	allNamespaces bool
	refreshSecs   int
	theme         string
)

var rootCmd = &cobra.Command{
	Use:   "kwatch",
	Short: "A terminal UI for monitoring Kubernetes services",
	Long:  "kwatch provides a rich terminal interface to monitor pods, deployments, and events on your Kubernetes cluster.",
	RunE:  run,
}

func init() {
	home, _ := os.UserHomeDir()
	defaultKubeconfig := filepath.Join(home, ".kube", "config")

	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig, "Path to kubeconfig file")
	rootCmd.Flags().StringVar(&kubeContext, "context", "", "Kubernetes context to use")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to watch (default: kubeconfig default)")
	rootCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Watch all namespaces")
	rootCmd.Flags().IntVar(&refreshSecs, "refresh", 5, "Refresh interval in seconds")
	rootCmd.Flags().StringVar(&theme, "theme", "github-dark", "Color theme (dracula, github-dark, everforest)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Apply theme
	if t, ok := tui.Themes[theme]; ok {
		tui.ApplyTheme(t)
	} else {
		return fmt.Errorf("unknown theme %q, available: dracula, github-dark, everforest", theme)
	}

	client, err := k8s.NewClient(kubeconfig, kubeContext)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ns := namespace
	if allNamespaces {
		ns = ""
	} else if ns == "" {
		// Resolve namespace from kubeconfig context (defaults to "default")
		ns = client.DefaultNamespace()
	}

	refreshInterval := time.Duration(refreshSecs) * time.Second

	app := tui.NewApp(client, ns, allNamespaces, refreshInterval, Version)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running kwatch: %w", err)
	}

	return nil
}
