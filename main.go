package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"net"
)

type entropySelectors struct {
	Fields []string `yaml:"fields"`
	Labels []string `yaml:"labels"`
}

type entropyConfig struct {
	NodeSelectors entropySelectors `yaml:"nodeSelectors"`
	PodSelectors  entropySelectors `yaml:"podSelectors"`
	Node          string           `yaml:"node"`
}

var ec entropyConfig
var inCluster bool

func combine(parts []string, separator string) (result string) {
	var buffer strings.Builder
	for _, element := range parts {
		if buffer.Len() > 0 {
			buffer.WriteString(separator)
		}
		buffer.WriteString(element)
	}
	result = buffer.String()
	return
}

func listSelectors(selectors entropySelectors) (listOptions metav1.ListOptions) {
	listOptions = metav1.ListOptions{}
	listOptions.FieldSelector = combine(selectors.Fields, ",")
	listOptions.LabelSelector = combine(selectors.Labels, ",")
	return
}

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
	listOptions := listSelectors(ec.NodeSelectors)
	for true {
		nodes, err := clientset.CoreV1().Nodes().List(listOptions)
		if err != nil {
			log.Printf("Cannot get a list of nodes. Skipping for now: %v\n", err)
		} else {
			log.Printf("%d nodes found\n", len(nodes.Items))
			// Make all node schedulable
			for i := 0; i < len(nodes.Items); i++ {
				node := nodes.Items[i]
				if node.Spec.Unschedulable == true {
					node.Spec.Unschedulable = false
					_, err = clientset.CoreV1().Nodes().Update(&node)
					if err != nil {
						log.Printf("Cannot uncordon the node: %v\n", err)
					}
				}
			}

			// And randomly unschedule one
			randomIndex := rand.Intn(len(nodes.Items))
			log.Printf("%d nodes found\n", len(nodes.Items))
			if len(nodes.Items) == 1 {
				log.Println("Only 1 node found, cannot cordon it off.")
			} else {
				for i := 0; i < len(nodes.Items); i++ {
					node := nodes.Items[i]
					if i == randomIndex {
						log.Printf("Cordoning off %s\n", node.Name)
						knode, err := clientset.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
						knode.Spec.Unschedulable = true
						_, err = clientset.CoreV1().Nodes().Update(knode)
						if err != nil {
							log.Printf("Cannot cordon the node: %v\n", err)
						}
					}
				}
			}
		}

		time.Sleep(5 * time.Minute)
	}
}

func killPods(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.PodSelectors)
	for true {
		pods, err := clientset.CoreV1().Pods("").List(listOptions)
		if err != nil {
			log.Println("Cannot get a list of running pods. Skipping for now.")
		} else {
			randomIndex := rand.Intn(len(pods.Items))
			for i := 0; i < len(pods.Items); i++ {
				if i == randomIndex {
					log.Printf("Force deleting pod %s.%s\n", pods.Items[i].Namespace, pods.Items[i].Name)
					err := clientset.CoreV1().Pods(pods.Items[i].Namespace).Delete(pods.Items[i].Name, metav1.NewDeleteOptions(0))
					if err != nil {
						log.Printf("Cannot delete a pod %s.%s\n", pods.Items[i].Namespace, pods.Items[i].Name)
					}
				}
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func monitor(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.PodSelectors)
	for true {
		services, err := clientset.CoreV1().Services("").List(listOptions)
		if err != nil {
			log.Println("Cannot get a list of services. Skipping for now.")
		}
		for i := 0; i < len(services.Items); i++ {
			service := services.Items[i]
			for _, element := range service.Spec.Ports {
				uri := service.Name + "." + service.Namespace + ".svc:" + element.TargetPort.String()
				if !inCluster {
					if element.NodePort > 0 {
						uri = ec.Node + ":" + strconv.Itoa(int(element.NodePort))
					} else {
						uri = ""
					}
				}

				if len(uri) > 0 {
					go func() {
						log.Printf("Connecting to %s\n", uri)
						conn, err := net.Dial(strings.ToLower(string(element.Protocol)), uri)
						if err != nil {
							log.Printf("could not connect to service %s: %v\n", uri, err)
						}
						defer conn.Close()
					}()
				}
			}
		}
		time.Sleep(2 * time.Second)
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
		inCluster = true
	}
	inCluster = false

	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, &ec)
	if err != nil {
		betterPanic(err.Error())
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
		go monitor(clientset)
		for true {
			time.Sleep(30 * time.Second)
		}
	}
}
