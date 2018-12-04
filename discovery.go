package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

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

type ApplicationState struct {
	Nodes     []NodeState    `yaml:"nodes"`
	Ingresses []IngressState `yaml:"ingresses"`
}

func discover(clientset *kubernetes.Clientset) {

	fmt.Printf("Saving everything.\n")
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
		appState.Nodes = append(appState.Nodes, NodeState{Name: node.Name})
	}

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

		appState.Ingresses = append(appState.Ingresses, IngressState{Name: ingress.Name, Namespace: ingress.Namespace, Endpoints: endpoints})
	}

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
	fmt.Printf("Saved everything.\n")

}
