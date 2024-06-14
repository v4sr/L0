package KubeClient

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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
