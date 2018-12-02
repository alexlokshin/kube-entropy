package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"net"
)

type EntropySelector struct {
	Fields   []string      `yaml:"fields"`
	Labels   []string      `yaml:"labels"`
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type ingressMonitoringConfig struct {
	Selector         EntropySelector `yaml:"selector"`
	DefaultHost      string          `yaml:"defaultHost"`
	Protocol         string          `yaml:"protocol"`
	Port             string          `yaml:"port"`
	SuccessHTTPCodes []string        `yaml:"successHttpCodes"`
}

type serviceMonitoringConfig struct {
	Selector     EntropySelector `yaml:"selector"`
	NodePortHost string          `yaml:"nodePortHost"`
}

type monitoringSettings struct {
	ServiceMonitoring serviceMonitoringConfig `yaml:"serviceMonitoring"`
	IngressMonitoring ingressMonitoringConfig `yaml:"ingressMonitoring"`
}

type entropyConfig struct {
	NodeChaos          EntropySelector    `yaml:"nodeChaos"`
	PodChaos           EntropySelector    `yaml:"podChaos"`
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

func listSelectors(selectors EntropySelector) (listOptions metav1.ListOptions) {
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

	nodes := &v1.NodeList{}
	var err error
	// Attempt to get a list of all scheduleable nodes
	for true {
		listOptions := listSelectors(ec.NodeChaos)
		nodes, err = clientset.CoreV1().Nodes().List(listOptions)
		if err != nil {
			log.Printf("ERROR: Cannot get a list of nodes. Skipping for now: %v\n", err)
			time.Sleep(time.Duration(1 * time.Minute))
			continue
		} else {
			log.Printf("%d nodes found\n", len(nodes.Items))
		}
		break
	}

	// Randomly make some of the node unschedulable
	for true {
		// Make all nodes schedulable
		for i := 0; i < len(nodes.Items); i++ {
			node := nodes.Items[i]
			if node.Spec.Unschedulable == true {
				node.Spec.Unschedulable = false
				_, err = clientset.CoreV1().Nodes().Update(&node)
				if err != nil {
					log.Printf("ERROR: Cannot uncordon the node: %v\n", err)
				}
			}
		}

		// And randomly unschedule one
		randomIndex := rand.Intn(len(nodes.Items))
		log.Printf("%d nodes found\n", len(nodes.Items))
		if len(nodes.Items) == 1 {
			log.Println("ERROR: Only 1 node found, cannot cordon it off.")
		} else {
			for i := 0; i < len(nodes.Items); i++ {
				node := nodes.Items[i]
				if i == randomIndex {
					log.Printf("Cordoning off %s\n", node.Name)
					node.Spec.Unschedulable = true
					_, err = clientset.CoreV1().Nodes().Update(&node)
					if err != nil {
						log.Printf("ERROR: Cannot cordon the node: %v\n", err)
					}
					// TODO: Drain the node
				}
			}
		}

		duration := time.Duration(rand.Int63n(ec.NodeChaos.Interval.Nanoseconds())) * time.Nanosecond
		log.Printf("For next node cordon sleeping for %s\n", duration)
		time.Sleep(duration)
	}
}

func killPods(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.PodChaos)
	for true {
		pods, err := clientset.CoreV1().Pods("").List(listOptions)
		if err != nil {
			log.Printf("ERROR: Cannot get a list of running pods. Skipping for now. %v\n", err)
		} else {
			randomIndex := rand.Intn(len(pods.Items))
			for i := 0; i < len(pods.Items); i++ {
				if i == randomIndex {
					log.Printf("Force deleting pod %s.%s\n", pods.Items[i].Namespace, pods.Items[i].Name)
					err := clientset.CoreV1().Pods(pods.Items[i].Namespace).Delete(pods.Items[i].Name, metav1.NewDeleteOptions(0))
					if err != nil {
						log.Printf("ERROR: Cannot delete a pod %s.%s: %v\n", pods.Items[i].Namespace, pods.Items[i].Name, err)
					}
				}
			}
		}

		duration := time.Duration(rand.Int63n(ec.PodChaos.Interval.Nanoseconds())) * time.Nanosecond
		log.Printf("For next pod deletion sleeping for %s\n", duration)
		time.Sleep(duration)
	}
}

func monitorServices(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.MonitoringSettings.ServiceMonitoring.Selector)
	for true {
		services, err := clientset.CoreV1().Services("").List(listOptions)
		if err != nil {
			log.Printf("ERROR: Cannot get a list of services. Skipping for now. %v\n", err)
		}
		for i := 0; i < len(services.Items); i++ {
			service := services.Items[i]
			if service.Spec.Type == v1.ServiceTypeExternalName {
				// This is just a proxy, it has no pods, nothing to test
				continue
			}

			for _, element := range service.Spec.Ports {
				uri := service.Name + "." + service.Namespace + ".svc:" + element.TargetPort.String()
				if !inCluster {
					// When testing out of cluster, only NodePort and ingress routes can be tested, LoadBalancer and ingresses are not supported yet
					if service.Spec.Type == v1.ServiceTypeNodePort {
						if element.NodePort > 0 {
							uri = ec.MonitoringSettings.ServiceMonitoring.NodePortHost + ":" + strconv.Itoa(int(element.NodePort))
						} else {
							uri = ""
						}
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
		time.Sleep(ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)
	}
}

func IsSuccessHTTPCode(validCodes []string, code string) (result bool) {
	for _, validCode := range validCodes {
		if len(code) == len(validCode) && strings.HasPrefix(code, strings.TrimRight(validCode, "x")) {
			return true
		}
	}
	return false
}

func monitorIngresses(clientset *kubernetes.Clientset) {
	listOptions := listSelectors(ec.MonitoringSettings.IngressMonitoring.Selector)
	for true {
		ingresses, err := clientset.Extensions().Ingresses("").List(listOptions)
		if err != nil {
			log.Printf("ERROR: Cannot get a list of ingresses. Skipping for now. %v\n", err)
		}
		for i := 0; i < len(ingresses.Items); i++ {
			ingress := ingresses.Items[i]

			for _, element := range ingress.Spec.Rules {
				host := ec.MonitoringSettings.IngressMonitoring.Protocol + "://" + ec.MonitoringSettings.IngressMonitoring.DefaultHost + ":" + ec.MonitoringSettings.IngressMonitoring.Port
				if len(strings.TrimSpace(element.Host)) > 0 {
					protocol := "https"
					if len(ec.MonitoringSettings.IngressMonitoring.Protocol) > 0 {
						protocol = ec.MonitoringSettings.IngressMonitoring.Protocol
					}
					port := "443"
					if len(ec.MonitoringSettings.IngressMonitoring.Port) > 0 {
						port = ec.MonitoringSettings.IngressMonitoring.Port
					}
					host = protocol + "://" + strings.TrimSpace(element.Host) + ":" + port
				}
				for _, path := range element.HTTP.Paths {
					uri := host + path.Path
					go func() {
						resp, err := http.Get(uri)
						if err != nil {
							// Timeout, DNS doesn't resolve, wrong protocol etc
							log.Printf("Cannot do http GET against %s.\n", uri)
						} else {
							if !IsSuccessHTTPCode(ec.MonitoringSettings.IngressMonitoring.SuccessHTTPCodes, strconv.Itoa(resp.StatusCode)) {
								log.Printf("Unexpected http code %d when calling %s.\n", resp.StatusCode, uri)
							}
						}
						defer resp.Body.Close()
					}()
				}
			}
		}
		time.Sleep(ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)
	}
}

func readConfig() {
	configFileData, err := ioutil.ReadFile("./config/config.yaml")
	if err != nil {
		log.Printf("ERROR: Config file cannot be read. #%v\n", err)
	}

	err = yaml.Unmarshal(configFileData, &ec)
	if err != nil {
		betterPanic(err.Error())
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

	readConfig()

	log.Printf("Starting kube-entropy.\n")
	rand.Seed(time.Now().UnixNano())
	log.Printf("Monitoring services every %s.\n", ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		betterPanic(err.Error())
	} else {
		log.Printf("Entropying it up.\n")
		if ec.PodChaos.Enabled {
			log.Printf("Launching the pod killer.\n")
			go killPods(clientset)
		}
		if ec.NodeChaos.Enabled {
			log.Printf("Launching the node killer.\n")
			go killNodes(clientset)
		}

		if ec.MonitoringSettings.ServiceMonitoring.Selector.Enabled {
			log.Printf("Launching the service monitor.\n")
			go monitorServices(clientset)
		}

		if ec.MonitoringSettings.IngressMonitoring.Selector.Enabled {
			log.Printf("Launching the ingress monitor.\n")
			go monitorIngresses(clientset)
		}

		for true {
			time.Sleep(30 * time.Second)
		}
	}
}
