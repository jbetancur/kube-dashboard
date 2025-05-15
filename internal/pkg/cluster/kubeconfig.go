package cluster

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubeConfig represents a Kubernetes configuration
type KubeConfig struct {
	Path         string
	LastModified time.Time
	Contexts     map[string]*api.Context
	RawConfig    *api.Config
}

// KubeConfigWatcher monitors kubeconfig file changes
type KubeConfigWatcher struct {
	config        *KubeConfig
	checkInterval time.Duration
	stopCh        chan struct{}
	mu            sync.RWMutex
	logger        *slog.Logger
	onChange      func(*KubeConfig)
}

// NewKubeConfigWatcher creates a new watcher instance
func NewKubeConfigWatcher(logger *slog.Logger, onChange func(*KubeConfig)) (*KubeConfigWatcher, error) {
	w := &KubeConfigWatcher{
		checkInterval: 30 * time.Second,
		stopCh:        make(chan struct{}),
		logger:        logger,
		onChange:      onChange,
	}

	// Initial loading of config
	config, err := w.loadKubeConfig()
	if err != nil {
		return nil, err
	}
	w.config = config

	return w, nil
}

// Start begins periodic checking of kubeconfig changes
func (w *KubeConfigWatcher) Start() {
	// Don't start if we're using in-cluster config
	if w.config == nil || w.config.Path == "" {
		w.logger.Info("Running in-cluster mode, kubeconfig watcher not started")
		return
	}

	w.logger.Info("Starting kubeconfig watcher",
		"path", w.config.Path,
		"interval", w.checkInterval)

	go func() {
		ticker := time.NewTicker(w.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.checkForChanges()
			case <-w.stopCh:
				w.logger.Info("Kubeconfig watcher stopped")
				return
			}
		}
	}()
}

// Stop halts the periodic checking
func (w *KubeConfigWatcher) Stop() {
	select {
	case <-w.stopCh:
		// Already closed
	default:
		close(w.stopCh)
	}
}

// GetConfig returns the current kubeconfig
func (w *KubeConfigWatcher) GetConfig() *KubeConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// checkForChanges checks if the kubeconfig file has been modified
func (w *KubeConfigWatcher) checkForChanges() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Skip if we're using in-cluster config
	if w.config == nil || w.config.Path == "" {
		return
	}

	info, err := os.Stat(w.config.Path)
	if err != nil {
		w.logger.Error("Failed to stat kubeconfig file", "error", err)
		return
	}

	// Check if modified
	if !info.ModTime().After(w.config.LastModified) {
		return // No changes
	}

	w.logger.Info("Kubeconfig file changed, reloading")

	// Reload config
	config, err := w.loadKubeConfig()
	if err != nil {
		w.logger.Error("Failed to reload kubeconfig", "error", err)
		return
	}

	w.config = config

	// Notify listeners
	if w.onChange != nil {
		w.onChange(config)
	}
}

// loadKubeConfig loads the kubeconfig file
func (w *KubeConfigWatcher) loadKubeConfig() (*KubeConfig, error) {
	// Check if running in cluster
	_, err := rest.InClusterConfig()
	if err == nil {
		w.logger.Info("Running in-cluster, not loading kubeconfig file")
		return &KubeConfig{
			Contexts:  map[string]*api.Context{"in-cluster": {}},
			RawConfig: &api.Config{},
		}, nil
	}

	w.logger.Info("Running outside cluster, loading kubeconfig file")

	// Get kubeconfig path
	kubeconfig, err := getKubeconfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig path: %w", err)
	}

	// Load the kubeconfig
	rawConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	info, err := os.Stat(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to stat kubeconfig file: %w", err)
	}

	return &KubeConfig{
		Path:         kubeconfig,
		LastModified: info.ModTime(),
		Contexts:     rawConfig.Contexts,
		RawConfig:    rawConfig,
	}, nil
}

// getKubeconfigPath gets the path to the current kubeconfig file
func getKubeconfigPath() (string, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return "", fmt.Errorf("kubeconfig file not found at %s", kubeconfig)
	}

	return kubeconfig, nil
}
