package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type NodeOptions struct {
	ListPods bool `short:"l" long:"list-pods" description:"List all pods in the node"`
}

type NodeCommand struct {
	Node     string      `positional-arg-name:"node" description:"Node name" required:"true"`
	NodeOpts NodeOptions `command:"" description:"Node options"`
}

type NamespaceOptions struct {
	ListPods        bool `short:"l" long:"list-pods" description:"List all pods in the namespace"`
	ListDeployments bool `long:"list-deploy" description:"List all deployments in the namespace"`
}

type NSCommand struct {
	Namespace string           `positional-arg-name:"namespace" description:"Namespace name" required:"true"`
	NSOpts    NamespaceOptions `command:"" description:"Namespace options"`
}

type Options struct {
	NodeCommand `command:"node" description:"Node options"`
	NSCommand   `command:"namespace" description:"Namespace options"`
	Kubeconfig  string `long:"kubeconfig" description:"Path to the kubeconfig file"`
}

func BuildClient(kubeconfig string) (*rest.Config, *kubernetes.Clientset, error) {
	restcfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error building ClientSet config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restcfg)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating ClientSet: %w", err)
	}

	return restcfg, clientset, nil
}

func nsExists(clientset kubernetes.Clientset, ns string) (bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("error with namespace %s: %w", ns, err)
	}
	return true, nil
}

func nodeExists(clientset kubernetes.Clientset, node string) (bool, error) {
	_, err := clientset.CoreV1().Nodes().Get(context.Background(), node, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("error with node %s: %w", node, err)
	}
	return true, nil
}

func getPo(clientset kubernetes.Clientset, ns_or_node_flag string, ns_or_node_name string) (*corev1.PodList, error) {
	var pods *corev1.PodList
	var err error

	switch ns_or_node_flag {
	case "namespace":
		check, err := nsExists(clientset, ns_or_node_name)
		if !check || err != nil {
			return nil, fmt.Errorf("namespace %s not available or existing: %w", ns_or_node_name, err)
		}

		pods, err = clientset.CoreV1().Pods(ns_or_node_name).List(context.Background(), metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + ns_or_node_name,
		})
		if err != nil {
			return nil, fmt.Errorf("error: %s", err)
		}
	case "node":
		check, err := nodeExists(clientset, ns_or_node_name)
		if !check || err != nil {
			return nil, fmt.Errorf("node %s not available or existing: %w", ns_or_node_name, err)
		}

		pods, err = clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + ns_or_node_name,
		})
		if err != nil {
			return nil, fmt.Errorf("error: %s", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("error retrieving pods: %w", err)
	}

	for _, pod := range pods.Items {
		fmt.Printf("%s -> %s\n", pod.Name, pod.Namespace)
	}

	return pods, nil
}

func getDeploy(clientset kubernetes.Clientset, ns_name string) (*appsv1.DeploymentList, error) {
	check, err := nsExists(clientset, ns_name)
	if !check || err != nil {
		return nil, fmt.Errorf("namespace %s not available or existing: %w", ns_name, err)
	}

	deploys, err := clientset.AppsV1().Deployments(ns_name).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error retrieving deployments: %w", err)
	}

	for _, deploy := range deploys.Items {
		fmt.Printf("%s -> %s\n", deploy.Name, deploy.Namespace)
	}

	return deploys, nil
}

func main() {
	var opts Options
	parser := flags.NewParser(&opts, flags.Default)

	_, err := parser.Parse()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("%s", parser.Usage)

	// Determine kubeconfig path
	kubeconfig := opts.Kubeconfig
	if kubeconfig == "" {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	_, clientset, err := BuildClient(kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	switch parser.Active.Name {
	case "node":
		if opts.NodeCommand.Node == "" {
			fmt.Println("Error: Please specify a node name.")
			os.Exit(1)
		}

		node := opts.NodeCommand.Node
		fmt.Printf("Node name: %s\n", node)
		if opts.NodeOpts.ListPods {
			_, err = getPo(*clientset, "node", node)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		}
	case "namespace":
		if opts.NSCommand.Namespace == "" {
			fmt.Println("Error: Please specify a namespace name")
			os.Exit(1)
		}

		namespace := opts.NSCommand.Namespace
		fmt.Printf("Namespace name: %s\n", namespace)
		if opts.NSCommand.NSOpts.ListPods {
			_, err = getPo(*clientset, "namespace", namespace)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		} else if opts.NSCommand.NSOpts.ListDeployments {
			_, err = getDeploy(*clientset, namespace)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		}
	}
}
