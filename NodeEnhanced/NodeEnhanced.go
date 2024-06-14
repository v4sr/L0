package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func printNode(clientset kubernetes.Clientset, node string) {
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, node := range nodeList.Items {
		fmt.Printf("Node: %s", node.Name)
	}
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
