package main

import (
	"fmt"
	"os"

	"io/ioutil"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
)

type NodeState struct {
	Name string `yaml:"name"`
}

type EndpointState struct {
	Url  string
	Code int
}

type ServiceState struct {
	Name      string
	Namespace string
	Endpoints []EndpointState
}

type ApplicationState struct {
	Nodes []NodeState `yaml:"nodes"`
}

func discover(clientset *kubernetes.Clientset) {
	fmt.Printf("Saving everything.\n")
	listOptions := listSelectors(ec.NodeChaos)
	nodes, err := clientset.CoreV1().Nodes().List(listOptions)
	if err != nil {
		betterPanic(err.Error())
	}

	listOptions = listSelectors(ec.MonitoringSettings.ServiceMonitoring.Selector)
	services, err := clientset.CoreV1().Services("").List(listOptions)
	if err != nil {
		betterPanic(err.Error())
	}

	listOptions = listSelectors(ec.MonitoringSettings.IngressMonitoring.Selector)
	ingresses, err := clientset.Extensions().Ingresses("").List(listOptions)
	if err != nil {
		betterPanic(err.Error())
	}

	appState := ApplicationState{}

	fmt.Printf("\nnodes:\n")
	for _, node := range nodes.Items {
		fmt.Printf("%s\n", node.Name)
		appState.Nodes = append(appState.Nodes, NodeState{Name: node.Name})
	}

	fmt.Printf("\nservices:\n")
	for _, service := range services.Items {
		fmt.Printf("%s.%s\n", service.Namespace, service.Name)
	}

	fmt.Printf("\ningresses:\n")
	for _, ingress := range ingresses.Items {
		fmt.Printf("%s.%s\n", ingress.Namespace, ingress.Name)
	}

	yml, err := yaml.Marshal(&appState)

	err = ioutil.WriteFile("./appstate.yml", yml, os.ModePerm)
	if err != nil {
		betterPanic("Cannot save appstate.yml.")
		return
	}
	fmt.Printf("Saved everything.\n")

}
