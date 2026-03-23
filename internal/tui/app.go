package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tituscarl/kwatch/internal/k8s"
)

// Messages
type PodsUpdatedMsg struct{ Pods []k8s.PodInfo }
type DeploymentsUpdatedMsg struct{ Deployments []k8s.DeploymentInfo }
type EventsUpdatedMsg struct{ Events []k8s.EventInfo }
type MetricsUpdatedMsg struct{ Metrics map[string]k8s.PodMetrics }
type TickMsg time.Time
type ErrorMsg struct{ Err error }
type LogsRefreshMsg struct{}

const (
	tabOverview = iota
	tabPods
	tabDeployments
	tabEvents
)

var tabNames = []string{"Overview", "Pods", "Deployments", "Events"}

type App struct {
	client          *k8s.Client
	namespace       string
	allNamespaces   bool
	refreshInterval time.Duration

	activeTab   int
	header      HeaderModel
	statusbar   StatusBarModel
	overview    OverviewModel
	pods        PodsModel
	deployments DeploymentsModel
	events      EventsModel
	detail      DetailModel
	showDetail  bool
	logs           LogsModel
	showLogs       bool
	logsCancelFunc context.CancelFunc // cancel the follow stream
	logsCh         <-chan string       // channel for follow stream lines
	podPicker      PodPickerModel
	showPodPicker  bool

	// Multi-pod log state
	multiPodLog  bool
	multiPodPods []k8s.PodInfo

	width  int
	height int
	err    error
}

func NewApp(client *k8s.Client, namespace string, allNS bool, refresh time.Duration) *App {
	info := client.ClusterInfo()
	metricsAvail := client.MetricsAvailable()

	return &App{
		client:          client,
		namespace:       namespace,
		allNamespaces:   allNS,
		refreshInterval: refresh,
		header:          NewHeaderModel(info, namespace, allNS),
		overview:        NewOverviewModel(),
		pods:            NewPodsModel(allNS, metricsAvail),
		deployments:     NewDeploymentsModel(allNS),
		events:          NewEventsModel(),
		detail:          NewDetailModel(),
		logs:            NewLogsModel(),
		podPicker:       NewPodPickerModel(),
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchPods(),
		a.fetchDeployments(),
		a.fetchEvents(),
		a.fetchMetrics(),
		a.tickCmd(),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.header.width = msg.Width
		a.statusbar.width = msg.Width
		contentHeight := a.contentHeight()
		a.pods.SetSize(msg.Width, contentHeight)
		a.deployments.SetSize(msg.Width, contentHeight)
		a.events.SetSize(msg.Width, contentHeight)
		a.overview.SetSize(msg.Width, contentHeight)
		a.detail.SetSize(msg.Width, contentHeight)
		a.logs.SetSize(msg.Width, contentHeight)
		a.podPicker.SetSize(msg.Width, contentHeight)

	case tea.KeyMsg:
		// If pod picker is open
		if a.showPodPicker {
			if key.Matches(msg, Keys.Escape) {
				a.showPodPicker = false
				a.statusbar.hidden = false
				return a, nil
			}
			if key.Matches(msg, Keys.Enter) {
				if a.podPicker.IsAllPodsSelected() {
					pods := a.podPicker.AllPods()
					if len(pods) == 0 {
						return a, nil
					}
					a.showPodPicker = false
					a.multiPodLog = true
					a.multiPodPods = pods
					a.logs.ShowMultiPod(a.podPicker.deploymentName, pods[0].Namespace, len(pods))
					a.showLogs = true
					return a, a.fetchLogs()
				} else if pod, ok := a.podPicker.SelectedPod(); ok {
					container := ""
					if len(pod.Containers) > 0 {
						container = pod.Containers[0].Name
					}
					a.showPodPicker = false
					a.multiPodLog = false
					a.multiPodPods = nil
					a.logs.Show(pod.Name, pod.Namespace, container)
					a.showLogs = true
					return a, a.fetchLogs()
				}
				return a, nil
			}
			a.podPicker.Update(msg)
			return a, nil
		}

		// If logs view is open, handle keys
		if a.showLogs {
			if key.Matches(msg, Keys.Escape) {
				// If grep is active, cancel it first
				if a.logs.HasActiveFilter() {
					a.logs.CancelFilter()
					return a, nil
				}
				a.stopFollowing()
				a.showLogs = false
				a.multiPodLog = false
				a.multiPodPods = nil
				a.statusbar.hidden = false
				return a, nil
			}
			prevFollowing := a.logs.following
			a.logs = a.logs.Update(msg)
			// Handle follow mode toggle
			if a.logs.following != prevFollowing {
				if a.logs.following {
					return a, a.startFollowing()
				}
				a.stopFollowing()
			}
			return a, nil
		}

		// If detail view is open, handle escape
		if a.showDetail {
			if key.Matches(msg, Keys.Escape) {
				a.showDetail = false
				return a, nil
			}
			a.detail, _ = a.detail.Update(msg)
			return a, nil
		}

		switch {
		case key.Matches(msg, Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, Keys.Tab1):
			a.activeTab = tabOverview
		case key.Matches(msg, Keys.Tab2):
			a.activeTab = tabPods
		case key.Matches(msg, Keys.Tab3):
			a.activeTab = tabDeployments
		case key.Matches(msg, Keys.Tab4):
			a.activeTab = tabEvents
		case key.Matches(msg, Keys.NextTab):
			a.activeTab = (a.activeTab + 1) % len(tabNames)
		case key.Matches(msg, Keys.PrevTab):
			a.activeTab = (a.activeTab - 1 + len(tabNames)) % len(tabNames)
		case key.Matches(msg, Keys.Enter):
			cmds = append(cmds, a.handleEnter())
		case key.Matches(msg, Keys.Logs):
			cmds = append(cmds, a.handleLogs())
		default:
			cmds = append(cmds, a.updateActiveTab(msg))
		}

	case PodsUpdatedMsg:
		a.pods.UpdatePods(msg.Pods)
		a.overview.UpdatePods(msg.Pods)
		a.statusbar.lastRefresh = time.Now()
		a.statusbar.err = nil

	case DeploymentsUpdatedMsg:
		a.deployments.UpdateDeployments(msg.Deployments)
		a.overview.UpdateDeployments(msg.Deployments)

	case EventsUpdatedMsg:
		a.events.UpdateEvents(msg.Events)
		a.overview.UpdateEvents(msg.Events)

	case MetricsUpdatedMsg:
		a.pods.UpdateMetrics(msg.Metrics)

	case DeploymentPodsMsg:
		if msg.Err != nil {
			a.statusbar.err = msg.Err
		} else {
			a.podPicker.UpdatePods(msg.Pods)
		}

	case LogsUpdatedMsg:
		a.logs.UpdateLogs(msg.Content, msg.Err)

	case LogLineMsg:
		if a.showLogs && a.logs.following {
			a.logs.AppendLines(msg.Lines)
			// Continue reading the next batch from the stream
			cmds = append(cmds, a.readNextLogLine())
		}

	case LogStreamEndedMsg:
		if a.showLogs && a.logs.following {
			a.logs.SetFollowing(false)
			if msg.Err != nil {
				a.logs.err = msg.Err
			}
		}

	case LogsRefreshMsg:
		if a.showLogs && !a.logs.following {
			cmds = append(cmds, a.fetchLogs())
		}

	case TickMsg:
		cmds = append(cmds,
			a.fetchPods(),
			a.fetchDeployments(),
			a.fetchEvents(),
			a.fetchMetrics(),
			a.tickCmd(),
		)
		// Refresh logs if open in snapshot mode
		if a.showLogs && !a.logs.following {
			cmds = append(cmds, a.fetchLogs())
		}

	case ErrorMsg:
		a.err = msg.Err
		a.statusbar.err = msg.Err
	}

	return a, tea.Batch(cmds...)
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	header := a.header.View()
	tabs := a.renderTabs()

	var content string
	if a.showPodPicker {
		content = a.podPicker.View()
	} else if a.showLogs {
		content = a.logs.View()
	} else if a.showDetail {
		content = a.detail.View()
	} else {
		switch a.activeTab {
		case tabOverview:
			content = a.overview.View()
		case tabPods:
			content = a.pods.View()
		case tabDeployments:
			content = a.deployments.View()
		case tabEvents:
			content = a.events.View()
		}
	}

	// Pad content to fill available height
	contentHeight := a.contentHeight()
	contentLines := lipgloss.Height(content)
	if contentLines < contentHeight {
		content = content + lipgloss.NewStyle().Height(contentHeight-contentLines).Render("")
	}

	statusbar := a.statusbar.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		tabs,
		content,
		statusbar,
	)
}

func (a *App) renderTabs() string {
	var tabs []string
	for i, name := range tabNames {
		shortcut := lipgloss.NewStyle().Foreground(colorDimText).Render(string(rune('1' + i)))
		if i == a.activeTab {
			tabs = append(tabs, ActiveTabStyle.Render(shortcut+" "+name))
		} else {
			tabs = append(tabs, InactiveTabStyle.Render(shortcut+" "+name))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return TabBarStyle.Width(a.width).Render(row)
}

func (a *App) contentHeight() int {
	// total height minus header(1), tab bar(2), status bar(1), some padding
	h := a.height - 5
	if h < 5 {
		h = 5
	}
	return h
}

func (a *App) handleEnter() tea.Cmd {
	switch a.activeTab {
	case tabPods:
		if pod, ok := a.pods.SelectedPod(); ok {
			a.detail.ShowPod(pod)
			a.showDetail = true
		}
	case tabDeployments:
		if dep, ok := a.deployments.SelectedDeployment(); ok {
			a.detail.ShowDeployment(dep)
			a.showDetail = true
		}
	}
	return nil
}

func (a *App) handleLogs() tea.Cmd {
	switch a.activeTab {
	case tabPods:
		pod, ok := a.pods.SelectedPod()
		if !ok {
			return nil
		}
		container := ""
		if len(pod.Containers) > 0 {
			container = pod.Containers[0].Name
		}
		a.logs.Show(pod.Name, pod.Namespace, container)
		a.showLogs = true
		a.statusbar.hidden = true
		return a.fetchLogs()

	case tabDeployments:
		dep, ok := a.deployments.SelectedDeployment()
		if !ok {
			return nil
		}
		a.podPicker.Show(dep.Name)
		a.showPodPicker = true
		a.statusbar.hidden = true
		return a.fetchDeploymentPods(dep.Namespace, dep.Name)
	}
	return nil
}

func (a *App) fetchDeploymentPods(namespace, name string) tea.Cmd {
	return func() tea.Msg {
		pods, err := a.client.ListDeploymentPods(namespace, name)
		if err != nil {
			return DeploymentPodsMsg{Err: err}
		}
		return DeploymentPodsMsg{Pods: pods}
	}
}

func (a *App) startFollowing() tea.Cmd {
	a.stopFollowing() // cancel any existing stream

	ctx, cancel := context.WithCancel(context.Background())
	a.logsCancelFunc = cancel

	client := a.client
	namespace := a.logs.namespace

	if a.multiPodLog {
		ch := make(chan string, 100*len(a.multiPodPods))
		a.logsCh = ch
		pods := a.multiPodPods
		go func() {
			_ = client.FollowMultiPodLogs(ctx, namespace, pods, 50, ch)
		}()
	} else {
		ch := make(chan string, 100)
		a.logsCh = ch
		podName := a.logs.podName
		container := a.logs.container
		go func() {
			_ = client.FollowPodLogs(ctx, namespace, podName, container, 50, ch)
		}()
	}

	// Return a cmd that reads one line at a time
	return a.readNextLogLine()
}

func (a *App) readNextLogLine() tea.Cmd {
	ch := a.logsCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		// Block on first line
		line, ok := <-ch
		if !ok {
			return LogStreamEndedMsg{}
		}
		// Drain all immediately available lines (non-blocking) into a batch
		batch := []string{line}
		for {
			select {
			case l, ok := <-ch:
				if !ok {
					// Channel closed, return what we have
					return LogLineMsg{Lines: batch}
				}
				batch = append(batch, l)
				if len(batch) >= 500 {
					// Cap batch size to keep UI responsive
					return LogLineMsg{Lines: batch}
				}
			default:
				// No more lines available right now
				return LogLineMsg{Lines: batch}
			}
		}
	}
}

func (a *App) stopFollowing() {
	if a.logsCancelFunc != nil {
		a.logsCancelFunc()
		a.logsCancelFunc = nil
	}
	a.logsCh = nil
}

func (a *App) fetchLogs() tea.Cmd {
	if a.multiPodLog {
		pods := a.multiPodPods
		namespace := a.logs.namespace
		return func() tea.Msg {
			content, err := a.client.GetMultiPodLogs(namespace, pods, 200)
			return LogsUpdatedMsg{Content: content, Err: err}
		}
	}
	podName := a.logs.podName
	namespace := a.logs.namespace
	container := a.logs.container
	return func() tea.Msg {
		content, err := a.client.GetPodLogs(namespace, podName, container, 200)
		return LogsUpdatedMsg{Content: content, Err: err}
	}
}

func (a *App) updateActiveTab(msg tea.Msg) tea.Cmd {
	switch a.activeTab {
	case tabPods:
		return a.pods.Update(msg)
	case tabDeployments:
		return a.deployments.Update(msg)
	case tabEvents:
		return a.events.Update(msg)
	}
	return nil
}

// Data fetching commands (read-only)

func (a *App) fetchPods() tea.Cmd {
	return func() tea.Msg {
		pods, err := a.client.ListPods(a.namespace)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return PodsUpdatedMsg{Pods: pods}
	}
}

func (a *App) fetchDeployments() tea.Cmd {
	return func() tea.Msg {
		deps, err := a.client.ListDeployments(a.namespace)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return DeploymentsUpdatedMsg{Deployments: deps}
	}
}

func (a *App) fetchEvents() tea.Cmd {
	return func() tea.Msg {
		events, err := a.client.ListEvents(a.namespace)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return EventsUpdatedMsg{Events: events}
	}
}

func (a *App) fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		metrics, err := a.client.GetPodMetrics(a.namespace)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return MetricsUpdatedMsg{Metrics: metrics}
	}
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(a.refreshInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
