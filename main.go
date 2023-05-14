package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//var ec entropyConfig

var dc discoveryConfig
var inCluster bool

func betterPanic(message string, args ...string) {
	temp := fmt.Sprintf(message, args)
	fmt.Printf("%s\n\n", temp)
	os.Exit(1)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func readTestPlan(configFileName string) (testPlan ApplicationState, err error) {
	configFileData, err := ioutil.ReadFile(configFileName)
	if err != nil {
		log.Printf("ERROR: Config file %s cannot be read. #%v\n", configFileName, err)
		return ApplicationState{}, err
	}

	err = yaml.Unmarshal(configFileData, &testPlan)
	if err != nil {
		return ApplicationState{}, err
	}
	return testPlan, nil
}

func readDiscoveryConfig(configFileName string) (dc discoveryConfig, err error) {
	configFileData, err := ioutil.ReadFile(configFileName)
	if err != nil {
		log.Printf("ERROR: Config file %s cannot be read. #%v\n", configFileName, err)
		return discoveryConfig{}, err
	}

	err = yaml.Unmarshal(configFileData, &dc)
	if err != nil {
		return discoveryConfig{}, err
	}
	return dc, nil
}

func main() {
	ctx := context.Background()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	testPlanFileName := flag.String("config", "./testplan.yaml", "Test plan file")
	discoveryConfigFileName := flag.String("dc", "./config/discovery.yaml", "Discovery file for the kube-entropy")

	mode := flag.String("mode", "chaos", "Runtime mode: chaos (default), discovery")
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

	if inCluster {
		log.Printf("Configured to run in in-cluster mode.\n")
	} else {
		log.Printf("Configured to run in out-of cluster mode.\nService testing other than NodePort is not supported.")
	}

	// TODO: Discovery mode

	log.Printf("Starting kube-entropy.\n")
	rand.Seed(time.Now().UnixNano())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		betterPanic(err.Error())
	} else {
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			betterPanic("ERROR: Unable to connect to the k8s cluster. " + err.Error())
		} else {
			log.Printf("Your cluster has a total of %d nodes.\n", len(nodes.Items))
		}

		if *mode == "chaos" {
			testPlan, err := readTestPlan(*testPlanFileName)
			if err != nil {
				betterPanic(err.Error())
			}

			log.Printf("Entropying it up.\n")
			if testPlan.Disruption.Pods.Enabled {
				log.Printf("Launching the pod killer.\n")
				go killPods(ctx, testPlan, clientset)
			}
			if testPlan.Disruption.Nodes.Enabled {
				log.Printf("Launching the node killer.\n")
				go killNodes(ctx, testPlan, clientset)
			}

			/*if inCluster {
				if ec.MonitoringSettings.ServiceMonitoring.Selector.Enabled {
					log.Printf("Launching the service monitor.\n")
					log.Printf("Monitoring services every %s.\n", ec.MonitoringSettings.ServiceMonitoring.Selector.Interval)

					go monitorServices(clientset)
				}
			}*/

			if testPlan.Monitoring.Enabled {
				log.Printf("Launching the ingress monitor.\n")
				log.Printf("Monitoring ingresses every %s.\n", testPlan.Monitoring.Interval)

				go monitorIngresses(testPlan)
			}

			for true {
				time.Sleep(30 * time.Second)
			}
		} else if *mode == "discovery" {
			log.Printf("Discovering the current configuration.\n")

			dc, err = readDiscoveryConfig(*discoveryConfigFileName)
			if err != nil {
				betterPanic(err.Error())
			}

			// Schedulable nodes
			// Services -- discover protocol
			// Ingresses -- look at the http response codes
			// Record to a config file
			discover(ctx, dc, clientset)
		} else if *mode == "dryrun" {
			// TODO: verify if test plan is still valid
			// TODO: add a resilient service for testing
			// TODO: Add a flakey service for testing

			testPlan, err := readTestPlan(*testPlanFileName)
			if err != nil {
				betterPanic(err.Error())
			}

			log.Printf("Verifying if ingresses match their constraints.\n")
			allValid := validateIngresses(testPlan)
			if allValid {
				log.Printf("Done. All valid.\n")
			} else {
				log.Printf("Done. Some errors.\n")
			}
		} else {
			betterPanic("Runtime mode not specified.")
		}
	}
}
