package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type entropySelector struct {
	Fields   []string      `yaml:"fields"`
	Labels   []string      `yaml:"labels"`
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type ingressMonitoringConfig struct {
	Selector         entropySelector `yaml:"selector"`
	DefaultHost      string          `yaml:"defaultHost"`
	Protocol         string          `yaml:"protocol"`
	Port             string          `yaml:"port"`
	SuccessHTTPCodes []string        `yaml:"successHttpCodes"`
}

type serviceMonitoringConfig struct {
	Selector     entropySelector `yaml:"selector"`
	NodePortHost string          `yaml:"nodePortHost"`
}

type monitoringSettings struct {
	ServiceMonitoring serviceMonitoringConfig `yaml:"serviceMonitoring"`
	IngressMonitoring ingressMonitoringConfig `yaml:"ingressMonitoring"`
}

type entropyConfig struct {
	NodeChaos          entropySelector    `yaml:"nodeChaos"`
	PodChaos           entropySelector    `yaml:"podChaos"`
	MonitoringSettings monitoringSettings `yaml:"monitoring"`
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

func listSelectors(selectors entropySelector) (listOptions metav1.ListOptions) {
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

func readConfig(configFileName string) {
	configFileData, err := ioutil.ReadFile(configFileName)
	if err != nil {
		log.Printf("ERROR: Config file %s cannot be read. #%v\n", configFileName, err)
		betterPanic("Shutting down.")
	}

	err = yaml.Unmarshal(configFileData, &ec)
	if err != nil {
		betterPanic(err.Error())
	}
}

func main() {
	configFileName := flag.String("config", "./config/config.yaml", "Configuration file for the kube-entropy.")
	flag.Parse()

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

	readConfig(*configFileName)
	if inCluster {
		log.Printf("Configured to run in in-cluster mode.\n")
	} else {
		log.Printf("Configured to run in out-of cluster mode.\nService testing other than NodePort is not supported.")
	}

	log.Printf("Starting kube-entropy.\n")
	rand.Seed(time.Now().UnixNano())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		betterPanic(err.Error())
	} else {
		nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			betterPanic(err.Error())
		} else {
			log.Printf("Your cluster has a total of %d nodes.\n", len(nodes.Items))
		}

		log.Printf("Entropying it up.\n")
		if ec.PodChaos.Enabled {
			log.Printf("Launching the pod killer.\n")
			go killPods(clientset)
		}
		if ec.NodeChaos.Enabled {
			log.Printf("Launching the node killer.\n")
			go killNodes(clientset)
		}

		if inCluster {
			if ec.MonitoringSettings.ServiceMonitoring.Selector.Enabled {
				log.Printf("Launching the service monitor.\n")
				log.Printf("Monitoring services every %s.\n", ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)

				go monitorServices(clientset)
			}
		}

		if ec.MonitoringSettings.IngressMonitoring.Selector.Enabled {
			log.Printf("Launching the ingress monitor.\n")
			log.Printf("Monitoring ingresses every %s.\n", ec.MonitoringSettings.IngressMonitoring.Selector.Interval)

			go monitorIngresses(clientset)
		}

		for true {
			time.Sleep(30 * time.Second)
		}
	}
}
