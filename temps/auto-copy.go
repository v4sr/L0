package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
  "strconv"
	"time"
)

const (
	NoFormat      = "\033[0m"
	CRed          = "\033[38;5;9m"
	CDarkOrange3  = "\033[38;5;166m"
	CGreen4       = "\033[38;5;28m"
)

func main() {
	if os.Geteuid() == 0 {
		fmt.Printf("%s [!] You must be a non-root user%s\n", CRed, NoFormat)
		os.Exit(1)
	}

	if len(os.Args) < 3 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Println("Uso:")
		fmt.Println("  auto-copy [NAMESPACE] [DATABASE]:")
		os.Exit(1)
	}

	SEL_NS := os.Args[1]
	today_date := time.Now().Format("060102")

	copy_dir_check := runSSHCommand(fmt.Sprintf("ls /media/migracion/%s | grep -c '%s-%s'", os.Getenv("USER"), SEL_NS, today_date))
	if strings.TrimSpace(copy_dir_check) == "0" {
		fmt.Printf("%s[!] Copy directory doesn't exists (/media/migracion/%s/%s-%s)%s\n", CRed, os.Getenv("USER"), SEL_NS, today_date, NoFormat)
		os.Exit(1)
	}

	TOTAL_PODS := runKubectlCommand(fmt.Sprintf("get po -n %s | tail +2 | wc -l", SEL_NS))
	fmt.Printf("Total pods: %s\n", strings.TrimSpace(TOTAL_PODS))

  totalPodsInt, err := strconv.Atoi(strings.TrimSpace(TOTAL_PODS))
  if err != nil {
    log.Fatalf("Error converting TOTAL_PODS to integer: %v", err)
  }

  //	for i := 0; i < strings.TrimSpace(TOTAL_PODS); i++ {
  for i := 0; i < totalPodsInt; i++ {
		pod_app := runKubectlCommand(fmt.Sprintf("get po -o jsonpath='{.items[%d].metadata.labels.app}' -n %s", i, SEL_NS))
		pod_app = strings.TrimSpace(pod_app)

		if pod_app == "odoo" {
			odoo_pod_name := runKubectlCommand(fmt.Sprintf("get po -o jsonpath='{.items[%d].metadata.name}' -n %s", i, SEL_NS))
			fmt.Printf("Odoo Pod name: %s\n", odoo_pod_name)
		}

		if pod_app == "postgres" {
			pg_pod_name := runKubectlCommand(fmt.Sprintf("get po -o jsonpath='{.items[%d].metadata.name}' -n %s", i, SEL_NS))
			fmt.Printf("Postgres Pod name: %s\n", pg_pod_name)
		}
	}

	odoo_pod_name := "odoo-pod-name" // Actualiza con el nombre correcto del pod Odoo
	pg_pod_name := "postgres-pod-name" // Actualiza con el nombre correcto del pod PostgreSQL

	DB_FAIL := runKubectlCommand(fmt.Sprintf("exec -it %s -n %s -- /bin/bash -c 'ls /var/lib/odoo/filestore | grep -c %s'", odoo_pod_name, SEL_NS, os.Args[2]))
	if DB_FAIL == "" {
		fmt.Printf("%s[!] Database doesn't exist%s\n", CRed, NoFormat)
		os.Exit(1)
	}

	SEL_DB := os.Args[2]

	runCommand("gnome-terminal", "--tab", "--", "bash", "-c", fmt.Sprintf("kubectl exec -it %s -n %s -- /bin/bash -c 'df -h'; bash", odoo_pod_name, SEL_NS))
	runCommand("gnome-terminal", "--tab", "--", "bash", "-c", fmt.Sprintf("kubectl exec -it %s -n %s -- /bin/bash -c 'df -h'; bash", pg_pod_name, SEL_NS))
	runCommand("gnome-terminal", "--tab", "--", "bash", "-c", fmt.Sprintf("ssh %s@nbsd190.nanobytes.es 'cd /media/migracion/%s/%s-%s'", os.Getenv("USER"), os.Getenv("USER"), SEL_NS, today_date))
	runCommand("gnome-terminal", "--tab", "--", "bash", "-c", fmt.Sprintf("kubectl exec -it %s -n %s -- /bin/bash -c 'apt update && apt install rsync openssh-client -y && rsync -avh /var/lib/odoo/filestore/%s %s@nbsd190.nanobytes.es:/media/migracion/%s/%s-%s'", odoo_pod_name, SEL_NS, SEL_DB, os.Getenv("USER"), os.Getenv("USER"), SEL_NS, today_date))
	runCommand("gnome-terminal", "--tab", "--", "bash", "-c", fmt.Sprintf("kubectl exec -it %s -n %s -- /bin/bash -c 'apt update && apt install rsync openssh-client -y && pg_dump -h localhost -U odoo -d %s | ssh %s@nbsd190.nanobytes.es 'cat > /media/migracion/%s/%s-%s/%s.dump''", odoo_pod_name, SEL_NS, SEL_DB, os.Getenv("USER"), os.Getenv("USER"), SEL_NS, today_date, SEL_DB))
}

func runCommand(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error executing command %s: %v\n%s", command, err, output)
	}
	return string(output)
}

func runSSHCommand(command string) string {
	return runCommand("ssh", command)
}

func runKubectlCommand(command string) string {
	return runCommand("kubectl", strings.Split(command, " ")...)
}

