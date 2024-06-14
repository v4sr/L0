package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
)

func BuildClient() (*rest.Config, *kubernetes.Clientset, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	restcfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("%w Error building ClientSet config", err)
	}

	clientset, err := kubernetes.NewForConfig(restcfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w Error creating ClientSet", err)
	}

	return restcfg, clientset, nil
}

func nsExists(clientset kubernetes.Clientset, selected_ns string) (bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), selected_ns, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("%w Error with namespace %s", err, selected_ns)
	}

	return true, nil
}

func getPo(clientset kubernetes.Clientset, selected_ns string) (*v1.Pod, error) {
	check, err := nsExists(clientset, selected_ns)
	if !check || err != nil {
		return nil, fmt.Errorf("%w Namespace %s non available or existing", err, selected_ns)
	}

	pods, err := clientset.CoreV1().Pods(selected_ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w Error getting pods from namespace %s", err, selected_ns)
	}

	fmt.Printf("Pods from %s namespace:\n", selected_ns)
	for i, pod := range pods.Items {
		fmt.Printf("[%d] %s\n", i, pod.Name)
	}

	fmt.Printf("Select a pod: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w Error scanning index", err)
	}

	selectedPodIndex, err := strconv.Atoi(scanner.Text())
	if err != nil {
		return nil, fmt.Errorf("%w Error converting string %s to int", err, scanner.Text())
	}

	total_pods := len(pods.Items)
	if selectedPodIndex < 0 || selectedPodIndex >= total_pods {
		return nil, fmt.Errorf("Invalid pod index (out of range [0-%d])", total_pods)
	}

	return &pods.Items[selectedPodIndex], nil
}

func getLogs(clientset kubernetes.Clientset, selected_pod v1.Pod, selected_ns string) (string, error) {
	count := int64(100)
	podLogOpts := v1.PodLogOptions{
		Follow:    true,
		TailLines: &count,
	}

	req := clientset.CoreV1().Pods(selected_ns).GetLogs(selected_pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("%w Error in opening stream", err)
	}

	defer podLogs.Close()

	for {
		buf := make([]byte, 2000)
		numBytes, err := podLogs.Read(buf)
		if numBytes == 0 {
			continue
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("%w Error reading logs", err)
		}
		message := string(buf[:numBytes])
		fmt.Print(message)
	}

	return "", nil
}

func executeRemoteCommand(restcfg *rest.Config, clientset kubernetes.Clientset, selected_pod *v1.Pod, command string) (string, string, error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	if command == "tty" {
		req := clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(selected_pod.Name).
			Namespace(selected_pod.Namespace).
			SubResource("exec").
			VersionedParams(&v1.PodExecOptions{
				Command: []string{"/bin/bash"},
				Stdin:   true,
				Stdout:  true,
				Stderr:  true,
				TTY:     true,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(restcfg, "POST", req.URL())
		if err != nil {
			return "", "", fmt.Errorf("failed to create executor: %w", err)
		}

		err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Tty:    true,
		})
		if err != nil {
			return "", "", fmt.Errorf("failed to execute command: %w", err)
		}

		return "", "", nil
	}

	request := clientset.CoreV1().RESTClient().
		Post().
		Namespace(selected_pod.Namespace).
		Resource("pods").
		Name(selected_pod.Name).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command: []string{"/bin/bash", "-c", command},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restcfg, "POST", request.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed executing command: %w", err)
	}

	return buf.String(), errBuf.String(), nil
}

func main() {
	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) == 0 {
		log.Fatal("use: ./krc <NAMESPACE>")
	}

	selected_ns := argsWithoutProg[0]

	restcfg, clientset, err := BuildClient()
	if err != nil {
		panic(err.Error())
	}

	selected_pod, err := getPo(*clientset, selected_ns)
	if err != nil {
		panic(err.Error())
	}

	if len(argsWithoutProg) == 1 {
		_, _, err := executeRemoteCommand(restcfg, *clientset, selected_pod, "tty")
		if err != nil {
			panic(err.Error())
		}
	} else if argsWithoutProg[1] == "-l" {
		_, err := getLogs(*clientset, *selected_pod, selected_ns)
		if err != nil {
			panic(err.Error())
		}
	} else {
		command_buf, command_err, err := executeRemoteCommand(restcfg, *clientset, selected_pod, argsWithoutProg[1])
		if err != nil {
			panic(err.Error())
		}

		if command_err != "" {
			panic(command_err)
		}

		fmt.Println(command_buf)
	}
}
