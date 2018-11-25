package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func betterPanic(message string) {
	fmt.Printf("%s\n\n", message)
	os.Exit(1)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func killNodes(clientset *kubernetes.Clientset) {
	//var cordonedNode *v1.Node
	for true {
		nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			log.Println("Cannot get a list of nodes. Skipping for now: %v", err)
		} else {
			log.Printf("%d nodes found\n", len(nodes.Items))
			// Make all node schedulable
			for i := 0; i < len(nodes.Items); i++ {
				node := nodes.Items[i]
				log.Printf("%s\n", node.Name)
				if node.Spec.Unschedulable == true {
					node.Spec.Unschedulable = false
					_, err = clientset.CoreV1().Nodes().Update(&node)
					if err != nil {
						log.Println("Cannot uncordon the node: %v", err)
					}
				}
			}
			time.Sleep(1 * time.Minute)

			// And randomly unschedule one
			randomIndex := rand.Intn(nodes.Size())
			log.Printf("%d nodes found\n", len(nodes.Items))
			for i := 0; i < len(nodes.Items); i++ {
				node := nodes.Items[i]
				log.Printf("%s\n", node.Name)
				if i == randomIndex {
					node.Spec.Unschedulable = true
					_, err = clientset.CoreV1().Nodes().Update(&node)
					if err != nil {
						log.Println("Cannot cordon the node: %v", err)
					}
				}
			}
		}

		time.Sleep(5 * time.Minute)
	}
}

func killPods(clientset *kubernetes.Clientset) {
	for i := 0; i < 1000; i++ {
		pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system,metadata.namespace!=docker"})
		if err != nil {
			log.Println("Cannot get a list of running pods. Skipping for now.")
		} else {
			randomIndex := rand.Intn(len(pods.Items))
			for i := 0; i < len(pods.Items); i++ {
				log.Printf("Force deleting pod %s.%s\n", pods.Items[i].Namespace, pods.Items[i].Name)
				if i == randomIndex {
					//clientset.CoreV1().Pods(pods.Items[i].Namespace).Delete(pods.Items[i].Name, metav1.NewDeleteOptions(0))
				}
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func main() {
	var kubeconfig *string
	home := homeDir()
	if home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Println("Local configuration not found, trying in-cluster configuration.")
		config, err = rest.InClusterConfig()
		if err != nil {
			betterPanic(err.Error())
		}
	}

	log.Printf("Starting kube-entropy.\n")
	rand.Seed(time.Now().UnixNano())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		betterPanic(err.Error())
	} else {
		log.Printf("Entropying it up.\n")
		go killPods(clientset)
		go killNodes(clientset)
		for true {
			time.Sleep(30 * time.Second)
		}
	}
}
