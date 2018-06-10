package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset_generated/clientset"
)

const (
	// TODO: Make configrable
	maxLength = 30
)

type NodeMetrics struct {
	Name  string
	Usage corev1.ResourceList
}

type NodeResources struct {
	Name        string
	Allocatable corev1.ResourceList
	Capacity    corev1.ResourceList
}

type PodMetrics struct {
	Name             string
	ContainerMetrics []ContainerMetrics
}

type PodResources struct {
	Name               string
	Namespace          string
	NodeName           string
	ContainerResources []ContainerResources
}

type ContainerMetrics struct {
	Name  string
	Usage corev1.ResourceList
}

type ContainerResources struct {
	Name             string
	ResourceRequests corev1.ResourceList
	ResourceLimits   corev1.ResourceList
}

func formatResourceList(rl corev1.ResourceList) string {
	return fmt.Sprintf("cpu=%s mem=%dMi", rl.Cpu().String(), rl.Memory().ScaledValue(resource.Mega))

}

type KubernetesResources struct {
	namespace string

	kubeClient    *kubernetes.Clientset
	metricsClient *metricsclientset.Clientset

	nodeMetrics   map[string]NodeMetrics
	nodeResources map[string]NodeResources

	podMetrics   map[string]PodMetrics
	podResources map[string]PodResources
}

func (kr *KubernetesResources) getNodeMetrics() error {

	metrics, err := kr.metricsClient.Metrics().NodeMetricses().List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "unable to get node metrics")
	}

	kr.nodeMetrics = make(map[string]NodeMetrics, len(metrics.Items))

	for _, n := range metrics.Items {
		kr.nodeMetrics[n.Name] = NodeMetrics{
			Name:  n.Name,
			Usage: n.Usage,
		}
	}

	return nil
}

func (kr *KubernetesResources) getNodeResources() error {
	nodes, err := kr.kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "unable to get node resources")
	}

	kr.nodeResources = make(map[string]NodeResources, len(nodes.Items))

	for _, node := range nodes.Items {
		kr.nodeResources[node.Name] = NodeResources{
			Name:        node.Name,
			Allocatable: node.Status.Allocatable,
			Capacity:    node.Status.Capacity,
		}

	}

	return nil

}

func (kr *KubernetesResources) getPodMetrics() error {

	metrics, err := kr.metricsClient.Metrics().PodMetricses(kr.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "unable to get pod metrics")
	}

	kr.podMetrics = make(map[string]PodMetrics, len(metrics.Items))
	for _, pod := range metrics.Items {
		pm := PodMetrics{
			Name:             pod.Name,
			ContainerMetrics: make([]ContainerMetrics, len(pod.Containers)),
		}
		for i, c := range pod.Containers {
			pm.ContainerMetrics[i] = ContainerMetrics{
				Name:  c.Name,
				Usage: c.Usage,
			}
		}
		kr.podMetrics[pod.Name] = pm
	}

	return nil
}

func (kr *KubernetesResources) getPodResources() error {
	pods, err := kr.kubeClient.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "unable to get pod resources")
	}

	kr.podResources = make(map[string]PodResources, len(pods.Items))

	for _, pod := range pods.Items {
		pr := PodResources{
			Name:               pod.Name,
			NodeName:           pod.Spec.NodeName,
			Namespace:          pod.Namespace,
			ContainerResources: make([]ContainerResources, len(pod.Spec.Containers)),
		}
		for i, c := range pod.Spec.Containers {
			pr.ContainerResources[i] = ContainerResources{
				Name:             c.Name,
				ResourceRequests: c.Resources.Requests,
				ResourceLimits:   c.Resources.Limits,
			}
		}

		kr.podResources[pod.Name] = pr
	}
	return nil
}

func (kr *KubernetesResources) Gather() {
	wg := sync.WaitGroup{}
	wg.Add(4)

	go func() {
		kr.getNodeMetrics()
		wg.Done()
	}()

	go func() {
		kr.getNodeResources()
		wg.Done()
	}()

	go func() {
		kr.getPodMetrics()
		wg.Done()
	}()

	go func() {
		kr.getPodResources()
		wg.Done()
	}()

	wg.Wait()

	frl := formatResourceList
	ml := func(s string) string {
		if len(s) > maxLength {
			return s[:maxLength/2] + "..." + s[len(s)-maxLength/2:]
		}
		return s
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Namespace", "Pod", "Container", "Usage", "Requests", "Limits"})
	data := [][]string{}

	nodeRequests := map[string][]corev1.ResourceList{}
	nodeLimits := map[string][]corev1.ResourceList{}

	for n := range kr.podMetrics {
		m := kr.podMetrics[n]
		r := kr.podResources[n]

		for i := range m.ContainerMetrics {
			cm := m.ContainerMetrics[i]
			cr := r.ContainerResources[i]
			data = append(data, []string{ml(r.Namespace), ml(n), ml(cm.Name), frl(cm.Usage), frl(cr.ResourceRequests), frl(cr.ResourceLimits)})

			nodeRequests[r.NodeName] = append(nodeRequests[r.NodeName], cr.ResourceRequests)
			nodeLimits[r.NodeName] = append(nodeLimits[r.NodeName], cr.ResourceLimits)
		}

	}

	table.AppendBulk(data)
	table.Render()

	tableNode := tablewriter.NewWriter(os.Stdout)
	tableNode.SetHeader([]string{"Node", "Usage", "Allocatable", "Resource Requests", "Resource Limits"})
	data = [][]string{}

	for n := range kr.nodeMetrics {
		m := kr.nodeMetrics[n]
		r := kr.nodeResources[n]

		cpuReqTotal := resource.Quantity{}
		memReqTotal := resource.Quantity{}
		for _, rl := range nodeRequests[n] {
			cpuReqTotal.Add(*rl.Cpu())
			memReqTotal.Add(*rl.Memory())
		}

		cpuLimTotal := resource.Quantity{}
		memLimTotal := resource.Quantity{}
		for _, rl := range nodeLimits[n] {
			cpuLimTotal.Add(*rl.Cpu())
			memLimTotal.Add(*rl.Memory())
		}

		// TODO(vishen): move these to function that formatResourceList also uses
		requestsString := fmt.Sprintf("cpu=%s mem=%dMi", cpuReqTotal.String(), memReqTotal.ScaledValue(resource.Mega))
		limitsString := fmt.Sprintf("cpu=%s mem=%dMi", cpuLimTotal.String(), memLimTotal.ScaledValue(resource.Mega))

		data = append(data, []string{ml(n), frl(m.Usage), frl(r.Allocatable), requestsString, limitsString})
	}

	tableNode.AppendBulk(data)
	tableNode.Render()

}

func main() {

	kubeConfig := ""
	kubeContext := ""
	namespace := ""

	// Determine kubeconfig path
	if kubeConfig == "" {
		if os.Getenv("KUBECONFIG") != "" {
			kubeConfig = os.Getenv("KUBECONFIG")
		} else {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}
	// Create the kubernetes client configuration
	clientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: kubeConfig,
		},
		&clientcmd.ConfigOverrides{
			CurrentContext: kubeContext,
		},
	).ClientConfig()
	if err != nil {
		log.Fatalf("unable to create k8s client config: %s\n", err)
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		log.Fatalf("unable to create k8s client: %s\n", err)
	}
	metricsClient, err := metricsclientset.NewForConfig(clientConfig)
	if err != nil {
		log.Fatalf("unable to create metrics client: %s\n", err)
	}

	kr := KubernetesResources{
		namespace:     namespace,
		kubeClient:    client,
		metricsClient: metricsClient,
	}

	kr.Gather()

}
