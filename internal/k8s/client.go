package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset       kubernetes.Interface
	dynamicClient   dynamic.Interface
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfigPath, lokiEndpoint, clusterName string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
	}, nil
}

// GetClientset returns the kubernetes clientset for external use
func (c *Client) GetClientset() kubernetes.Interface {
	return c.clientset
}

// GetDynamicClient returns the dynamic client for CR operations
func (c *Client) GetDynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// GetDaemonSetStatus returns the status of the OTEL Collector DaemonSet
func (c *Client) GetDaemonSetStatus(ctx context.Context, name, namespace string) (interface{}, error) {
	daemonSet, err := c.clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get DaemonSet %s/%s: %w", namespace, name, err)
	}

	status := map[string]interface{}{
		"name":                name,
		"namespace":           namespace,
		"desired_nodes":       daemonSet.Status.DesiredNumberScheduled,
		"current_nodes":       daemonSet.Status.CurrentNumberScheduled,
		"ready_nodes":         daemonSet.Status.NumberReady,
		"updated_nodes":       daemonSet.Status.UpdatedNumberScheduled,
		"available_nodes":     daemonSet.Status.NumberAvailable,
		"unavailable_nodes":   daemonSet.Status.NumberUnavailable,
		"creation_timestamp":  daemonSet.CreationTimestamp,
	}

	return status, nil
}