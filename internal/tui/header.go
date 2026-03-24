package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type HeaderModel struct {
	clusterInfo k8s.ClusterInfo
	namespace   string
	allNS       bool
	width       int
}

func NewHeaderModel(info k8s.ClusterInfo, namespace string, allNS bool) HeaderModel {
	return HeaderModel{
		clusterInfo: info,
		namespace:   namespace,
		allNS:       allNS,
	}
}

func (h HeaderModel) View() string {
	logo := HeaderStyle.Render(" kwatch ")

	cluster := HeaderLabelStyle.Render("cluster:") + " " + HeaderValueStyle.Render(h.clusterInfo.ClusterName)
	ctx := HeaderLabelStyle.Render("ctx:") + " " + HeaderValueStyle.Render(h.clusterInfo.ContextName)

	nsDisplay := h.namespace
	if h.allNS {
		nsDisplay = "all"
	} else if nsDisplay == "" {
		nsDisplay = "default"
	}
	ns := HeaderLabelStyle.Render("ns:") + " " + HeaderValueStyle.Render(nsDisplay)

	bgStyle := lipgloss.NewStyle().
		Width(h.width).
		Background(lipgloss.Color("#1A1A2E")).
		Padding(0, 1)

	line1 := logo + "  " + cluster
	line2 := "          " + ctx + "  " + ns

	return bgStyle.Render(line1 + "\n" + line2)
}
