package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type HeaderModel struct {
	clusterInfo k8s.ClusterInfo
	namespace   string
	allNS       bool
	version     string
	width       int
}

func NewHeaderModel(info k8s.ClusterInfo, namespace string, allNS bool, version string) HeaderModel {
	return HeaderModel{
		clusterInfo: info,
		namespace:   namespace,
		allNS:       allNS,
		version:     version,
	}
}

func (h HeaderModel) View() string {
	logoStyle := lipgloss.NewStyle().
		Foreground(colorPurple).
		Bold(true)

	logo := logoStyle.Render("╦╔═ ╦ ╦ ╔═╗ ╔╦╗ ╔═╗ ╦ ╦") + "\n" +
		logoStyle.Render("╠╩╗ ║║║ ╠═╣  ║  ║   ╠═╣") + "\n" +
		logoStyle.Render("╩ ╚ ╚╩╝ ╩ ╩  ╩  ╚═╝ ╩ ╩")

	ver := lipgloss.NewStyle().Foreground(colorDimText).Render(" " + h.version)
	logoWithVer := lipgloss.JoinHorizontal(lipgloss.Bottom, logo, ver)

	cluster := HeaderLabelStyle.Render("cluster:") + " " + HeaderValueStyle.Render(h.clusterInfo.ClusterName)
	ctx := HeaderLabelStyle.Render("ctx:") + " " + HeaderValueStyle.Render(h.clusterInfo.ContextName)

	nsDisplay := h.namespace
	if h.allNS {
		nsDisplay = "all"
	} else if nsDisplay == "" {
		nsDisplay = "default"
	}
	ns := HeaderLabelStyle.Render("ns:") + " " + HeaderValueStyle.Render(nsDisplay)

	header := logoWithVer + "\n" + cluster + "\n" + ctx + "\n" + ns

	wrapStyle := lipgloss.NewStyle().
		Width(h.width).
		Padding(0, 1)

	return wrapStyle.Render(header)
}
