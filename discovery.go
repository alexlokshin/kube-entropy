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

type EndpointState struct {
	URL         string            `yaml:"url"`
	Method      string            `yaml:"method"`
	Headers     map[string]string `yaml:"headers"`
	Code        int               `yaml:"code"`
	PodSelector map[string]string
}

type IngressState struct {
	Name      string          `yaml:"name"`
	Namespace string          `yaml:"namespace"`
	Endpoints []EndpointState `yaml:"endpoints"`
}

type NodeConfiguration struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	Items    []string      `yaml:"items"`
}

type IngressConfiguration struct {
	Enabled          bool           `yaml:"enabled"`
	Interval         time.Duration  `yaml:"interval"`
	SuccessHTTPCodes []string       `yaml:"successHttpCodes"`
	Items            []IngressState `yaml:"ingresses"`
}

type ApplicationState struct {
	Nodes     NodeConfiguration    `yaml:"nodes"`
	Ingresses IngressConfiguration `yaml:"ingresses"`
}

func discover(dc discoveryConfig, clientset *kubernetes.Clientset) {

	fmt.Printf("Creating a test plan.\n")
	listOptions := listSelectors(dc.Nodes)
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

	appState := ApplicationState{
		Nodes: NodeConfiguration{
			Enabled:  dc.Nodes.Enabled,
			Interval: dc.Nodes.Interval},
		Ingresses: IngressConfiguration{
			Enabled:          dc.Ingress.Selector.Enabled,
			Interval:         dc.Ingress.Selector.Interval,
			SuccessHTTPCodes: dc.Ingress.SuccessHTTPCodes}}

	fmt.Printf("\nnodes:\n")
	for _, node := range nodes.Items {
		fmt.Printf("%s\n", node.Name)
		appState.Nodes.Items = append(appState.Nodes.Items, node.Name)
	}

	// Ingress points to a service, service points to Deployments/DaemonSets
	fmt.Printf("\ningresses:\n")
	for _, ingress := range ingresses.Items {
		fmt.Printf("%s.%s\n", ingress.Namespace, ingress.Name)
		endpoints := []EndpointState{}
		for _, rule := range ingress.Spec.Rules {

			host := getIngressHost(dc, ingress, rule)
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
					var headers = map[string]string{}
					for key := range resp.Header {
						if key != "Date" && key != "Content-Length" && key != "Set-Cookie" {
							headers[key] = resp.Header.Get(key)
						}
					}

					endpoints = append(endpoints, EndpointState{URL: uri, Method: "GET", Code: statusCode, Headers: headers, PodSelector: service.Spec.Selector})
					defer resp.Body.Close()
				}

			}
		}

		appState.Ingresses.Items = append(appState.Ingresses.Items, IngressState{Name: ingress.Name, Namespace: ingress.Namespace, Endpoints: endpoints})
	}

	fmt.Printf("\nservices:\n")
	for _, service := range services.Items {
		fmt.Printf("%s.%s\n", service.Namespace, service.Name)
	}

	yml, err := yaml.Marshal(&appState)

	testPlanFileName := "./testplan.yaml"

	err = ioutil.WriteFile(testPlanFileName, yml, os.ModePerm)
	if err != nil {
		betterPanic("Cannot save %s.", testPlanFileName)
		return
	}
	fmt.Printf("Test plan saved as %s.\n", testPlanFileName)

}
