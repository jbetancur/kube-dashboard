package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterManager struct {
	clusters    map[string]*Cluster
	podEvents   map[string][]interface{}
	connections map[string][]*websocket.Conn
	mutex       sync.RWMutex
}

type Cluster struct {
	clientset *kubernetes.Clientset
	stopCh    chan struct{}
	informer  informers.SharedInformerFactory
}

func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		clusters:    make(map[string]*Cluster),
		podEvents:   make(map[string][]interface{}),
		connections: make(map[string][]*websocket.Conn),
	}
}

func (cm *ClusterManager) AddCluster(clusterID, kubeconfigPath string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	stopCh := make(chan struct{})
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute*5)

	// Start watching namespaces (extendable for other resources)
	informerFactory.Core().V1().Namespaces().Informer()

	// Start watching pods (extendable for other resources)
	podInformer := informerFactory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				fmt.Printf("[Cluster %s] Pod added: %v\n", clusterID, obj)
				cm.mutex.Lock()
				cm.podEvents[clusterID] = append(cm.podEvents[clusterID], obj)
				for _, conn := range cm.connections[clusterID] {
					if err := conn.WriteJSON(obj); err != nil {
						fmt.Printf("Error sending pod event: %v\n", err)
					}
				}
				cm.mutex.Unlock()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				fmt.Printf("[Cluster %s] Pod updated: %v\n", clusterID, newObj)
				cm.mutex.Lock()
				for _, conn := range cm.connections[clusterID] {
					if err := conn.WriteJSON(newObj); err != nil {
						fmt.Printf("Error sending pod event: %v\n", err)
					}
				}
				cm.mutex.Unlock()
			},
			DeleteFunc: func(obj interface{}) {
				fmt.Printf("[Cluster %s] Pod deleted: %v\n", clusterID, obj)
				cm.mutex.Lock()
				for _, conn := range cm.connections[clusterID] {
					if err := conn.WriteJSON(obj); err != nil {
						fmt.Printf("Error sending pod event: %v\n", err)
					}
				}
				cm.mutex.Unlock()
			},
		},
	)

	// Store cluster and start informer
	cluster := &Cluster{
		clientset: clientset,
		stopCh:    stopCh,
		informer:  informerFactory,
	}
	cm.clusters[clusterID] = cluster
	go informerFactory.Start(stopCh)

	fmt.Printf("Cluster %s added and watching resources.\n", clusterID)
	return nil
}

func (cm *ClusterManager) RemoveCluster(clusterID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cluster, exists := cm.clusters[clusterID]; exists {
		close(cluster.stopCh)
		delete(cm.clusters, clusterID)
		delete(cm.podEvents, clusterID)
		for _, conn := range cm.connections[clusterID] {
			conn.Close()
		}
		delete(cm.connections, clusterID)

		fmt.Printf("Cluster %s removed.\n", clusterID)
	} else {
		fmt.Printf("Cluster %s not found.\n", clusterID)
	}
}

func (cm *ClusterManager) GetPodEvents(clusterID string) []interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.podEvents[clusterID]
}

func (cm *ClusterManager) GetAllPodsFromCache(clusterID, namespace string) ([]v1.Pod, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	cluster, exists := cm.clusters[clusterID]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	podInformer := cluster.informer.Core().V1().Pods().Informer()
	pods := make([]v1.Pod, 0)

	for _, obj := range podInformer.GetStore().List() {
		pod := obj.(*v1.Pod)
		if pod.Namespace == namespace {
			pods = append(pods, *pod)
		}
	}

	// Sort pods by name
	sort.Slice(pods, func(i, j int) bool {
		podI := pods[i]
		podJ := pods[j]
		return podI.Name < podJ.Name
	})

	return pods, nil
}

func (cm *ClusterManager) StreamPodLogs(clusterID, namespace, podName, containerName string, conn *websocket.Conn) error {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	cluster, exists := cm.clusters[clusterID]
	if !exists {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	podLogOpts := v1.PodLogOptions{
		Follow: true,
	}
	if containerName != "" {
		podLogOpts.Container = containerName
	}

	req := cluster.clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("error in opening stream: %v", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	for {
		_, err := io.Copy(buf, podLogs)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error in copying information from podLogs to buf: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
			return fmt.Errorf("error sending log message: %v", err)
		}
		buf.Reset()
	}
	return nil
}

func (cm *ClusterManager) GetNamespaces(clusterID string) ([]v1.Namespace, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	cluster, exists := cm.clusters[clusterID]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}

	namespaceInformer := cluster.informer.Core().V1().Namespaces().Informer()
	namespaces := make([]v1.Namespace, 0)

	for _, obj := range namespaceInformer.GetStore().List() {
		namespace := obj.(*v1.Namespace)
		namespaces = append(namespaces, *namespace)
	}

	// namespaceList, err := cluster.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to list namespaces: %v", err)
	// }

	// namespaces := make([]v1.Namespace, len(namespaceList.Items))
	// for i, ns := range namespaceList.Items {
	// 	namespaces[i] = ns
	// }

	return namespaces, nil
}

func (cm *ClusterManager) AddConnection(clusterID string, conn *websocket.Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[clusterID] = append(cm.connections[clusterID], conn)
}

func main() {
	cm := NewClusterManager()

	// Example usage (replace with actual kubeconfig paths)
	go cm.AddCluster("cluster-1", "./kubeconfig-1")
	// go cm.AddCluster("cluster-2", "./kubeconfig-2")

	// time.Sleep(10 * time.Second) // Simulate runtime
	defer cm.RemoveCluster("cluster-1")
	// cm.RemoveCluster("cluster-2")

	app := fiber.New()

	app.Post("/add-cluster", func(c *fiber.Ctx) error {
		clusterID := c.Query("id")
		kubeconfigPath := c.Query("kubeconfig")
		if err := cm.AddCluster(clusterID, kubeconfigPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/remove-cluster", func(c *fiber.Ctx) error {
		clusterID := c.Query("id")
		cm.RemoveCluster(clusterID)
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/clusters", func(c *fiber.Ctx) error {
		cm.mutex.RLock()
		defer cm.mutex.RUnlock()
		clusters := make([]string, 0, len(cm.clusters))
		for id := range cm.clusters {
			clusters = append(clusters, id)
		}
		return c.JSON(clusters)
	})

	app.Get("/podevents/:clusterID", func(c *fiber.Ctx) error {
		clusterID := c.Params("clusterID")
		events := cm.GetPodEvents(clusterID)
		return c.JSON(events)
	})

	app.Get("/pods/:clusterID/:namespace", func(c *fiber.Ctx) error {
		clusterID := c.Params("clusterID")
		namespace := c.Params("namespace")
		pods, err := cm.GetAllPodsFromCache(clusterID, namespace)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return c.JSON(pods)
	})

	app.Get("/namespaces/:clusterID", func(c *fiber.Ctx) error {
		clusterID := c.Params("clusterID")
		namespaces, err := cm.GetNamespaces(clusterID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return c.JSON(namespaces)
	})

	app.Get("/ws/podlogs/:clusterID/:namespace/:podName/:containerName?", websocket.New(func(c *websocket.Conn) {
		clusterID := c.Params("clusterID")
		namespace := c.Params("namespace")
		podName := c.Params("podName")
		containerName := c.Params("containerName")
		if err := cm.StreamPodLogs(clusterID, namespace, podName, containerName, c); err != nil {
			fmt.Printf("Error streaming pod logs: %v\n", err)
			c.Close()
		}
	}))

	app.Get("/ws/:clusterID", websocket.New(func(c *websocket.Conn) {
		clusterID := c.Params("clusterID")
		cm.AddConnection(clusterID, c)
		defer c.Close()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	fmt.Println("Starting server on :8081")
	app.Listen(":8081")
}
