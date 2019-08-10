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

type PodConfiguration struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type IngressConfiguration struct {
	SuccessHTTPCodes []string       `yaml:"successHttpCodes"`
	Items            []IngressState `yaml:"routes"`
}

type MonitoringConfiguration struct {
	Enabled   bool                 `yaml:"enabled"`
	Interval  time.Duration        `yaml:"interval"`
	Ingresses IngressConfiguration `yaml:"ingresses"`
}

type DisruptionConfiguration struct {
	Nodes NodeConfiguration `yaml:"nodes"`
	Pods  PodConfiguration  `yaml:"pods"`
}

type ApplicationState struct {
	Disruption DisruptionConfiguration `yaml:"disruption"`
	Monitoring MonitoringConfiguration `yaml:"monitoring"`
}

func discover(dc discoveryConfig, clientset *kubernetes.Clientset) {

	fmt.Printf("Creating a test plan.\n")
	listOptions := listSelectors(dc.Nodes)
	nodes, err := clientset.CoreV1().Nodes().List(listOptions)
	if err != nil {
		betterPanic(err.Error())
	}

	ingresses, err := clientset.ExtensionsV1beta1().Ingresses("").List(listSelectors(dc.Ingress.Selector))
	if err != nil {
		betterPanic(err.Error())
	}

	appState := ApplicationState{
		Disruption: DisruptionConfiguration{
			Nodes: NodeConfiguration{
				Enabled:  dc.Nodes.Enabled,
				Interval: dc.Nodes.Interval},
			Pods: PodConfiguration{
				Enabled:  dc.Pods.Enabled,
				Interval: dc.Pods.Interval,
			},
		},
		Monitoring: MonitoringConfiguration{
			Enabled:  dc.Ingress.Selector.Enabled,
			Interval: dc.Ingress.Selector.Interval,
			Ingresses: IngressConfiguration{

				SuccessHTTPCodes: dc.Ingress.SuccessHTTPCodes},
		},
	}

	fmt.Printf("\nnodes:\n")
	for _, node := range nodes.Items {
		fmt.Printf("%s\n", node.Name)
		appState.Disruption.Nodes.Items = append(appState.Disruption.Nodes.Items, node.Name)
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

				if uri[len(uri)-1] != '/' {
					uri = uri + "/"
				}

				resp, err := http.Get(uri)
				if err != nil {
					// Timeout, DNS doesn't resolve, wrong protocol etc
					log.Printf("Cannot do http GET against %s.\n", uri)
				} else {
					statusCode := resp.StatusCode
					var headers = map[string]string{}
					for key := range resp.Header {
						if key != "Date" && key != "Content-Length" && key != "Set-Cookie" && key != "Etag" && key != "Last-Modified" {
							headers[key] = resp.Header.Get(key)
						}
					}

					if statusCode == 503 || statusCode == 502 {
						fmt.Printf("Got a %d from %s.\n", statusCode, uri)
					} else {
						endpoints = append(endpoints, EndpointState{URL: uri, Method: "GET", Code: statusCode, Headers: headers, PodSelector: service.Spec.Selector})
					}
					defer resp.Body.Close()
				}

			}
		}

		if len(endpoints) == 0 {
			fmt.Printf("No endpoints available for %s.%s.\n", ingress.Namespace, ingress.Name)
		} else {
			appState.Monitoring.Ingresses.Items = append(appState.Monitoring.Ingresses.Items, IngressState{Name: ingress.Name, Namespace: ingress.Namespace, Endpoints: endpoints})
		}
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
