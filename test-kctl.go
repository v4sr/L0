package k8s

import (
	"flag"
	"log"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func ExecBashExample(client kubernetes.Interface, config *restclient.Config, podName string,
	command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := []string{
		"/bin/bash",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podname).
		Namespace("amadueno-acsource-demo-v17ee").SubResource("exec")
	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	if stdin == nil {
		option.Stdin = false
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remoteCommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	/** ClientSet **/

	// ClientSet from Outside
	config, err := clientcmd.BuildConfigFromFlags("", "/home/amadueno/.kube/config")
	if err != nil {
		return fmt.Errorf("Fail to build the rke2 config. Error - %s", err)
	}

	// ClientSet from Inside
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("Fail to build the rke2 config. Error - %s", err)
	}

	// Build the ClientSet
	ClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Fail to create the k8s client set. Errorf - %s", err)
	}

	// Inorder to create the dynamic ClientSet
	dynamicClientSet, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Fail to create the dynamic client set. Errorf - %s", err)
	}

}
