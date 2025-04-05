package main

import (
	"context"
	"fmt"
	"os"
	"plugin"

	"log/slog"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jbetancur/dashboard/internal/pkg/config"
	"github.com/jbetancur/dashboard/internal/pkg/core"
	"github.com/jbetancur/dashboard/internal/pkg/messaging"
	"github.com/jbetancur/dashboard/internal/pkg/providers"
)

type state int

const (
	stateClusters state = iota
	stateNamespaces
	statePods
)

type listItem string

func (i listItem) Title() string       { return string(i) }
func (i listItem) Description() string { return "" }
func (i listItem) FilterValue() string { return string(i) }

type model struct {
	state             state
	clusters          []providers.ClusterConfig
	namespaces        []string
	pods              []string
	selectedCluster   string
	selectedNamespace string
	list              list.Model
	clusterManager    *core.ClusterManager
	namespaceManager  *core.NamespaceManager
	podManager        *core.PodManager
	width             int
	height            int
}

func initialModel(clusterManager *core.ClusterManager, namespaceManager *core.NamespaceManager, podManager *core.PodManager, clusters []providers.ClusterConfig) model {
	// Initialize the list with clusters
	items := []list.Item{}
	for _, cluster := range clusters {
		items = append(items, listItem(cluster.ID))
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Clusters"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return model{
		state:            stateClusters,
		clusters:         clusters,
		clusterManager:   clusterManager,
		namespaceManager: namespaceManager,
		podManager:       podManager,
		list:             l,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "enter":
			switch m.state {
			case stateClusters:
				// Move to namespaces
				m.selectedCluster = string(m.list.SelectedItem().(listItem))

				// Fetch namespaces using NamespaceManager
				namespaces, err := m.namespaceManager.ListNamespaces(context.Background(), m.selectedCluster)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error fetching namespaces: %v\n", err)
					return m, tea.Quit
				}

				// Update the list with namespaces
				items := []list.Item{}
				for _, ns := range namespaces {
					items = append(items, listItem(ns.Name))
				}
				m.list.SetItems(items)
				m.list.Title = fmt.Sprintf("Namespaces in %s", m.selectedCluster)
				m.state = stateNamespaces

			case stateNamespaces:
				// Move to pods
				m.selectedNamespace = string(m.list.SelectedItem().(listItem))

				// Fetch pods using PodManager
				pods, err := m.podManager.ListPods(context.Background(), m.selectedCluster, m.selectedNamespace)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error fetching pods: %v\n", err)
					return m, tea.Quit
				}

				// Update the list with pods
				items := []list.Item{}
				for _, pod := range pods {
					items = append(items, listItem(pod.Name))
				}
				m.list.SetItems(items)
				m.list.Title = fmt.Sprintf("Pods in %s/%s", m.selectedCluster, m.selectedNamespace)
				m.state = statePods
			}
		case "backspace":
			switch m.state {
			case stateNamespaces:
				// Go back to clusters
				items := []list.Item{}
				for _, cluster := range m.clusters {
					items = append(items, listItem(cluster.ID))
				}
				m.list.SetItems(items)
				m.list.Title = "Clusters"
				m.state = stateClusters
			case statePods:
				// Go back to namespaces
				items := []list.Item{}
				for _, ns := range m.namespaces {
					items = append(items, listItem(ns))
				}
				m.list.SetItems(items)
				m.list.Title = fmt.Sprintf("Namespaces in %s", m.selectedCluster)
				m.state = stateNamespaces
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-6) // Adjust for header/footer and borders
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.list.View()
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger) // Set as the default logger

	// Load configuration
	appConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return
	}

	// Load provider plugins
	var clusterProvider providers.Provider
	for _, providerConfig := range appConfig.Providers {
		clusterProvider, err = loadProviderPlugin(providerConfig.Path, providerConfig.Config, logger)
		if err != nil {
			logger.Error("Failed to load provider plugin", "name", providerConfig.Name, "error", err)
			return
		}
		logger.Info("Loaded provider plugin", "name", providerConfig.Name)
	}

	// Discover clusters
	clusters, err := clusterProvider.DiscoverClusters()
	if err != nil {
		logger.Error("Error discovering clusters", "error", err)
		return
	}

	// Initialize the message queue (e.g., gRPC, Kafka, RabbitMQ)
	messageQueue := messaging.NewGRPCMessageQueue()
	defer messageQueue.Close()

	// Create the shared EventPublisher
	eventPublisher := core.NewEventPublisher(messageQueue)

	// Register subscribers for topics
	err = messageQueue.Subscribe("pod_events", func(message []byte) error {
		// logger.Info("Received pod event", "message", string(message))
		// Process the pod event (e.g., store it in a database)
		return nil
	})
	if err != nil {
		logger.Error("Failed to subscribe to pod_events", "error", err)
		return
	}

	err = messageQueue.Subscribe("namespace_events", func(message []byte) error {
		// logger.Info("Received namespace event", "message", string(message))
		// Process the namespace event (e.g., store it in a database)
		return nil
	})
	if err != nil {
		logger.Error("Failed to subscribe to namespace_events", "error", err)
		return
	}

	clusterManager := core.NewClusterManager(logger, clusterProvider)

	// Add clusters to the ClusterManager
	for _, cluster := range clusters {
		if err := clusterManager.RegisterCluster(cluster.ID); err != nil {
			logger.Error("Error adding cluster", "clusterID", cluster.ID, "error", err)
		}
	}

	namespaceManager, err := core.NewNamespaceManager(eventPublisher, clusterManager, clusters)
	if err != nil {
		logger.Error("Failed to initialize NamespaceManager", "error", err)
		return
	}

	podManager, err := core.NewPodManager(eventPublisher, clusterManager, clusters)
	if err != nil {
		logger.Error("Failed to initialize PodManager", "error", err)
		return
	}

	// Start the TUI
	p := tea.NewProgram(initialModel(clusterManager, namespaceManager, podManager, clusters), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}

func loadProviderPlugin(path string, config map[string]string, logger *slog.Logger) (providers.Provider, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symbol, err := p.Lookup("New")
	if err != nil {
		return nil, fmt.Errorf("failed to find 'New' function in plugin: %w", err)
	}

	newFunc, ok := symbol.(func(map[string]string, *slog.Logger) providers.Provider)
	if !ok {
		return nil, fmt.Errorf("invalid 'New' function signature in plugin")
	}

	return newFunc(config, logger), nil
}
