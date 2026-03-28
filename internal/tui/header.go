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
	logoStyle := lipgloss.NewStyle().
		Foreground(colorPurple).
		Bold(true)

	logo := logoStyle.Render("╦╔═ ╦ ╦ ╔═╗ ╔╦╗ ╔═╗ ╦ ╦") + "\n" +
		logoStyle.Render("╠╩╗ ║║║ ╠═╣  ║  ║   ╠═╣") + "\n" +
		logoStyle.Render("╩ ╚ ╚╩╝ ╩ ╩  ╩  ╚═╝ ╩ ╩")

	cluster := HeaderLabelStyle.Render("cluster:") + " " + HeaderValueStyle.Render(h.clusterInfo.ClusterName)
	ctx := HeaderLabelStyle.Render("ctx:") + " " + HeaderValueStyle.Render(h.clusterInfo.ContextName)

	nsDisplay := h.namespace
	if h.allNS {
		nsDisplay = "all"
	} else if nsDisplay == "" {
		nsDisplay = "default"
	}
	ns := HeaderLabelStyle.Render("ns:") + " " + HeaderValueStyle.Render(nsDisplay)

	header := logo + "\n" + cluster + "\n" + ctx + "\n" + ns

	wrapStyle := lipgloss.NewStyle().
		Width(h.width).
		Padding(0, 1)

	return wrapStyle.Render(header)
}
