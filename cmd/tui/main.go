package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jbetancur/dashboard/internal/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

var (
	titleStyle = lipgloss.NewStyle().
			MarginLeft(2).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{
			Light: "#04B575",
			Dark:  "#04B575",
		})

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{
			Light: "#FF0000",
			Dark:  "#FF0000",
		})

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("170")).
				Bold(true)
)

// ViewType represents the current view being displayed
type ViewType int

const (
	ClusterView ViewType = iota
	NamespaceView
	PodView
	DetailView
)

// KeyMap defines the keybindings for the application
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Back      key.Binding
	Quit      key.Binding
	Refresh   key.Binding
	Delete    key.Binding
	Describe  key.Binding
	Logs      key.Binding
	Help      key.Binding
	ClusterNS key.Binding
}

var keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Describe: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "describe"),
	),
	Logs: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "logs"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	ClusterNS: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "change namespace"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Up, k.Down, k.Enter, k.Back, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.Back, k.Refresh, k.Quit},
		{k.Delete, k.Describe, k.Logs},
		{k.ClusterNS, k.Help},
	}
}

// Model represents the application state
type Model struct {
	currentView       ViewType
	clusterTable      table.Model
	namespaceTable    table.Model
	podTable          table.Model
	detailView        viewport.Model
	help              help.Model
	keys              KeyMap
	width             int
	height            int
	selectedCluster   string
	selectedNamespace string
	selectedPod       string
	statusMessage     string
	errorMessage      string
	clientManager     *client.ClientManager
	showHelp          bool
	loading           bool
}

// Message types
type clientLoadedMsg struct {
	clientManager *client.ClientManager
}

type clustersLoadedMsg struct {
	rows []table.Row
}

type namespacesLoadedMsg struct {
	rows []table.Row
}

type podsLoadedMsg struct {
	rows []table.Row
}

type podDetailsLoadedMsg struct {
	content string
}

type errorMsg struct {
	err error
}

func initialModel() Model {
	// Initialize tables with empty data
	clusterTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 20},
			{Title: "API URL", Width: 40},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	clusterTable.SetStyles(table.Styles{
		Selected: selectedRowStyle,
	})

	namespaceTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 20},
			{Title: "Status", Width: 10},
			{Title: "Age", Width: 10},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	namespaceTable.SetStyles(table.Styles{
		Selected: selectedRowStyle,
	})

	podTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 30},
			{Title: "Ready", Width: 10},
			{Title: "Status", Width: 10},
			{Title: "Restarts", Width: 10},
			{Title: "Age", Width: 10},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	podTable.SetStyles(table.Styles{
		Selected: selectedRowStyle,
	})

	detailView := viewport.New(80, 20)
	detailView.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	return Model{
		currentView:    ClusterView,
		clusterTable:   clusterTable,
		namespaceTable: namespaceTable,
		podTable:       podTable,
		detailView:     detailView,
		help:           help.New(),
		keys:           keys,
		statusMessage:  "Loading clusters...",
		loading:        true,
		showHelp:       false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		initializeClient(),
		tea.EnterAltScreen,
	)
}

// Helper functions to format data
func formatAge(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return duration.HumanDuration(time.Since(timestamp.Time))
}

// initializeClient uses your existing client manager
func initializeClient() tea.Cmd {
	return func() tea.Msg {
		// Create a logger that writes to our log file
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

		// Create client manager using your existing code
		clientManager, err := client.NewClientManager(logger) // Pass the logger here instead of nil
		if err != nil {
			return errorMsg{err: err}
		}
		return clientLoadedMsg{clientManager: clientManager}
	}
}

// Commands for loading data
func loadClusters(clientManager *client.ClientManager) tea.Cmd {
	return func() tea.Msg {
		kubeClients := clientManager.GetClients()
		rows := make([]table.Row, 0, len(kubeClients))

		for _, c := range kubeClients {
			apiURL := ""
			if c.Config != nil {
				apiURL = c.Config.Host
			}
			rows = append(rows, table.Row{c.Cluster, apiURL})
		}

		return clustersLoadedMsg{rows: rows}
	}
}

func loadNamespaces(clientManager *client.ClientManager, clusterID string) tea.Cmd {
	return func() tea.Msg {
		client, exists := clientManager.GetClient(clusterID)
		if !exists {
			return errorMsg{err: fmt.Errorf("cluster %s not found", clusterID)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		namespaces, err := client.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to list namespaces: %w", err)}
		}

		rows := make([]table.Row, 0, len(namespaces.Items))
		for _, ns := range namespaces.Items {
			age := formatAge(ns.CreationTimestamp)
			rows = append(rows, table.Row{ns.Name, string(ns.Status.Phase), age})
		}

		return namespacesLoadedMsg{rows: rows}
	}
}

func loadPods(clientManager *client.ClientManager, clusterID, namespace string) tea.Cmd {
	return func() tea.Msg {
		client, exists := clientManager.GetClient(clusterID)
		if !exists {
			return errorMsg{err: fmt.Errorf("cluster %s not found", clusterID)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pods, err := client.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to list pods: %w", err)}
		}

		rows := make([]table.Row, 0, len(pods.Items))
		for _, pod := range pods.Items {
			// Calculate readiness
			ready := 0
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Ready {
					ready++
				}
			}
			readyStr := fmt.Sprintf("%d/%d", ready, len(pod.Spec.Containers))

			// Calculate restarts
			restarts := 0
			for _, containerStatus := range pod.Status.ContainerStatuses {
				restarts += int(containerStatus.RestartCount)
			}

			age := formatAge(pod.CreationTimestamp)

			rows = append(rows, table.Row{
				pod.Name,
				readyStr,
				string(pod.Status.Phase),
				fmt.Sprintf("%d", restarts),
				age,
			})
		}

		return podsLoadedMsg{rows: rows}
	}
}

func loadPodDetails(clientManager *client.ClientManager, clusterID, namespace, podName string) tea.Cmd {
	return func() tea.Msg {
		client, exists := clientManager.GetClient(clusterID)
		if !exists {
			return errorMsg{err: fmt.Errorf("cluster %s not found", clusterID)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pod, err := client.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to get pod: %w", err)}
		}

		// Format detailed info
		content := fmt.Sprintf("Pod: %s\n", pod.Name)
		content += fmt.Sprintf("Namespace: %s\n", pod.Namespace)
		content += fmt.Sprintf("Node: %s\n", pod.Spec.NodeName)
		content += fmt.Sprintf("Status: %s\n", pod.Status.Phase)
		content += fmt.Sprintf("IP: %s\n", pod.Status.PodIP)
		content += fmt.Sprintf("Created: %s\n\n", pod.CreationTimestamp.Format(time.RFC3339))

		content += "Labels:\n"
		for k, v := range pod.Labels {
			content += fmt.Sprintf("  %s: %s\n", k, v)
		}
		content += "\n"

		content += "Containers:\n"
		for i, container := range pod.Spec.Containers {
			content += fmt.Sprintf("  [%d] %s\n", i+1, container.Name)
			content += fmt.Sprintf("      Image: %s\n", container.Image)

			// Add status if available
			for _, status := range pod.Status.ContainerStatuses {
				if status.Name == container.Name {
					content += fmt.Sprintf("      Ready: %t\n", status.Ready)
					content += fmt.Sprintf("      Restarts: %d\n", status.RestartCount)

					if status.State.Running != nil {
						content += fmt.Sprintf("      State: Running (started %s)\n",
							formatAge(status.State.Running.StartedAt))
					} else if status.State.Waiting != nil {
						content += fmt.Sprintf("      State: Waiting (%s)\n", status.State.Waiting.Reason)
					} else if status.State.Terminated != nil {
						content += fmt.Sprintf("      State: Terminated (%s)\n", status.State.Terminated.Reason)
					}
				}
			}

			// Add resource requests/limits
			if container.Resources.Limits != nil || container.Resources.Requests != nil {
				content += "      Resources:\n"
				if container.Resources.Requests != nil {
					cpu := container.Resources.Requests.Cpu()
					memory := container.Resources.Requests.Memory()
					content += fmt.Sprintf("        Requests: %s CPU, %s memory\n", cpu.String(), memory.String())
				}
				if container.Resources.Limits != nil {
					cpu := container.Resources.Limits.Cpu()
					memory := container.Resources.Limits.Memory()
					content += fmt.Sprintf("        Limits: %s CPU, %s memory\n", cpu.String(), memory.String())
				}
			}
			content += "\n"
		}

		return podDetailsLoadedMsg{content: content}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 6 // Title + status + padding
		footerHeight := 3 // Help view + padding
		tableHeight := m.height - headerHeight - footerHeight

		m.clusterTable.SetHeight(tableHeight)
		m.namespaceTable.SetHeight(tableHeight)
		m.podTable.SetHeight(tableHeight)
		m.detailView.Height = tableHeight
		m.detailView.Width = m.width - 4

		m.help.Width = m.width

	case clientLoadedMsg:
		m.clientManager = msg.clientManager
		return m, loadClusters(m.clientManager)

	case clustersLoadedMsg:
		m.clusterTable.SetRows(msg.rows)
		m.statusMessage = fmt.Sprintf("Loaded %d clusters", len(msg.rows))
		m.loading = false

	case namespacesLoadedMsg:
		m.namespaceTable.SetRows(msg.rows)
		m.statusMessage = fmt.Sprintf("Loaded %d namespaces", len(msg.rows))
		m.loading = false

	case podsLoadedMsg:
		m.podTable.SetRows(msg.rows)
		m.statusMessage = fmt.Sprintf("Loaded %d pods", len(msg.rows))
		m.loading = false

	case podDetailsLoadedMsg:
		m.detailView.SetContent(msg.content)
		m.statusMessage = "Loaded pod details"
		m.loading = false

	case errorMsg:
		m.errorMessage = msg.err.Error()
		m.loading = false

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.loading {
			// Don't process key events while loading
			return m, nil
		}

		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		if key.Matches(msg, m.keys.Refresh) {
			// Refresh the current view
			switch m.currentView {
			case ClusterView:
				m.loading = true
				m.statusMessage = "Refreshing clusters..."
				return m, loadClusters(m.clientManager)
			case NamespaceView:
				m.loading = true
				m.statusMessage = "Refreshing namespaces..."
				return m, loadNamespaces(m.clientManager, m.selectedCluster)
			case PodView:
				m.loading = true
				m.statusMessage = "Refreshing pods..."
				return m, loadPods(m.clientManager, m.selectedCluster, m.selectedNamespace)
			case DetailView:
				m.loading = true
				m.statusMessage = "Refreshing pod details..."
				return m, loadPodDetails(m.clientManager, m.selectedCluster, m.selectedNamespace, m.selectedPod)
			}
		}

		// Handle navigation based on current view
		switch m.currentView {
		case ClusterView:
			switch {
			case key.Matches(msg, m.keys.Enter):
				if len(m.clusterTable.Rows()) == 0 {
					return m, nil
				}

				selectedRow := m.clusterTable.SelectedRow()
				m.selectedCluster = selectedRow[0] // Cluster name
				m.currentView = NamespaceView
				m.statusMessage = "Loading namespaces..."
				m.loading = true

				return m, loadNamespaces(m.clientManager, m.selectedCluster)
			}

		case NamespaceView:
			switch {
			case key.Matches(msg, m.keys.Back):
				m.currentView = ClusterView
				m.selectedNamespace = ""
				return m, nil
			case key.Matches(msg, m.keys.Enter):
				if len(m.namespaceTable.Rows()) == 0 {
					return m, nil
				}

				selectedRow := m.namespaceTable.SelectedRow()
				m.selectedNamespace = selectedRow[0] // Namespace name
				m.currentView = PodView
				m.statusMessage = "Loading pods..."
				m.loading = true

				return m, loadPods(m.clientManager, m.selectedCluster, m.selectedNamespace)
			}

		case PodView:
			switch {
			case key.Matches(msg, m.keys.Back):
				m.currentView = NamespaceView
				m.selectedPod = ""
				return m, nil
			case key.Matches(msg, m.keys.Enter):
				if len(m.podTable.Rows()) == 0 {
					return m, nil
				}

				selectedRow := m.podTable.SelectedRow()
				m.selectedPod = selectedRow[0] // Pod name
				m.currentView = DetailView
				m.statusMessage = "Loading pod details..."
				m.loading = true

				return m, loadPodDetails(m.clientManager, m.selectedCluster, m.selectedNamespace, m.selectedPod)
			}

		case DetailView:
			if key.Matches(msg, m.keys.Back) {
				m.currentView = PodView
				return m, nil
			}
		}

		// Update the appropriate table/viewport based on current view
		switch m.currentView {
		case ClusterView:
			m.clusterTable, cmd = m.clusterTable.Update(msg)
			cmds = append(cmds, cmd)
		case NamespaceView:
			m.namespaceTable, cmd = m.namespaceTable.Update(msg)
			cmds = append(cmds, cmd)
		case PodView:
			m.podTable, cmd = m.podTable.Update(msg)
			cmds = append(cmds, cmd)
		case DetailView:
			m.detailView, cmd = m.detailView.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m Model) View() string {
	// Show help or main view
	if m.showHelp {
		return "\n" + m.help.View(m.keys)
	}

	var content string

	// Title bar based on current view
	title := "Kubernetes Dashboard"
	switch m.currentView {
	case ClusterView:
		title += " - Clusters"
	case NamespaceView:
		title += fmt.Sprintf(" - Namespaces (Cluster: %s)", m.selectedCluster)
	case PodView:
		title += fmt.Sprintf(" - Pods (Namespace: %s)", m.selectedNamespace)
	case DetailView:
		title += fmt.Sprintf(" - Pod Details: %s", m.selectedPod)
	}

	// Show main content based on current view
	switch m.currentView {
	case ClusterView:
		content = m.clusterTable.View()
	case NamespaceView:
		content = m.namespaceTable.View()
	case PodView:
		content = m.podTable.View()
	case DetailView:
		content = m.detailView.View()
	}

	// Status bar
	status := " "
	if m.errorMessage != "" {
		status = errorMessageStyle.Render("Error: " + m.errorMessage)
	} else if m.loading {
		status = statusMessageStyle.Render("Loading...")
	} else if m.statusMessage != "" {
		status = statusMessageStyle.Render(m.statusMessage)
	}

	// Help hint at the bottom
	helpHint := "Press ? for help | q to quit | r to refresh"

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Top,
		titleStyle.Render(title),
		"\n",
		content,
		"\n",
		status,
		helpHint,
	)
}

func main() {
	// Set up logging to a file
	f, err := os.Create("tui.log")
	if err != nil {
		fmt.Println("Could not create log file:", err)
		os.Exit(1)
	}
	defer f.Close()
	log.SetOutput(f)

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Println("Error running program:", err)
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
