package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"io/ioutil"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type NodeState struct {
	Name string `yaml:"name"`
}

type EndpointState struct {
	Url         string `yaml:"url"`
	Method      string `yaml:"method"`
	ContentType string `yaml:"contentType"`
	Code        int    `yaml:"code"`
	PodSelector map[string]string
}

type IngressState struct {
	Name      string          `yaml:"name"`
	Namespace string          `yaml:"namespace"`
	Endpoints []EndpointState `yaml:"endpoints"`
}

type NodeConfiguration struct {
	Items    []string      `yaml:"items"`
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type IngressConfiguration struct {
	Items    []IngressState `yaml:"ingresses"`
	Enabled  bool           `yaml:"enabled"`
	Interval time.Duration  `yaml:"interval"`
}

type ApplicationState struct {
	Nodes     NodeConfiguration    `yaml:"nodes"`
	Ingresses IngressConfiguration `yaml:"ingresses"`
}

func discover(dc discoveryConfig, clientset *kubernetes.Clientset) {

	fmt.Printf("Creating a test plan.\n")
	listOptions := listSelectors(ec.NodeChaos)
	nodes, err := clientset.CoreV1().Nodes().List(listOptions)
	if err != nil {
		betterPanic(err.Error())
	}

	services, err := clientset.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		betterPanic(err.Error())
	}

	ingresses, err := clientset.Extensions().Ingresses("").List(metav1.ListOptions{})
	if err != nil {
		betterPanic(err.Error())
	}

	appState := ApplicationState{}

	fmt.Printf("\nnodes:\n")
	for _, node := range nodes.Items {
		fmt.Printf("%s\n", node.Name)
		appState.Nodes.Items = append(appState.Nodes.Items, node.Name)
	}
	appState.Nodes.Enabled = dc.Nodes.Enabled
	appState.Nodes.Interval = dc.Nodes.Interval

	// Ingress points to a service, service points to Deployments/DaemonSets
	fmt.Printf("\ningresses:\n")
	for _, ingress := range ingresses.Items {
		fmt.Printf("%s.%s\n", ingress.Namespace, ingress.Name)
		endpoints := []EndpointState{}
		for _, rule := range ingress.Spec.Rules {

			host := getIngressHost(ingress, rule)
			for _, path := range rule.HTTP.Paths {

				serviceName := path.Backend.ServiceName
				service, err := clientset.CoreV1().Services(ingress.Namespace).Get(serviceName, metav1.GetOptions{})
				if err != nil {
					log.Printf("Cannot get a service %s.\n", serviceName)
					continue
				}

				uri := host + path.Path

				resp, err := http.Get(uri)
				if err != nil {
					// Timeout, DNS doesn't resolve, wrong protocol etc
					log.Printf("Cannot do http GET against %s.\n", uri)
				} else {
					statusCode := resp.StatusCode
					contentType := resp.Header.Get("Content-Type")
					endpoints = append(endpoints, EndpointState{Url: uri, Method: "GET", Code: statusCode, ContentType: contentType, PodSelector: service.Spec.Selector})
					defer resp.Body.Close()
				}

			}
		}

		appState.Ingresses.Items = append(appState.Ingresses.Items, IngressState{Name: ingress.Name, Namespace: ingress.Namespace, Endpoints: endpoints})
	}
	appState.Ingresses.Enabled = dc.Ingress.Selector.Enabled
	appState.Ingresses.Interval = dc.Ingress.Selector.Interval

	fmt.Printf("\nservices:\n")
	for _, service := range services.Items {
		fmt.Printf("%s.%s\n", service.Namespace, service.Name)
	}

	yml, err := yaml.Marshal(&appState)

	err = ioutil.WriteFile("./appstate.yml", yml, os.ModePerm)
	if err != nil {
		betterPanic("Cannot save appstate.yml.")
		return
	}
	fmt.Printf("Test plan saved as appstate.yml.\n")

}
