package main

import (
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

type discoveryConfig struct {
	Nodes   entropySelector         `yaml:"nodes"`
	Ingress ingressMonitoringConfig `yaml:"ingresses"`
}

func combine(parts []string, separator string) (result string) {
	/*var buffer strings.Builder
	for _, element := range parts {
		if buffer.Len() > 0 {
			buffer.WriteString(separator)
		}
		buffer.WriteString(element)
	}
	result = buffer.String()
	return*/
	result = combineWithPrefix(parts, "", separator)
	return
}

func combineWithPrefix(parts []string, prefix string, separator string) (result string) {
	var buffer strings.Builder
	for _, element := range parts {
		if buffer.Len() > 0 {
			buffer.WriteString(separator)
		}
		buffer.WriteString(prefix)
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

func namedNodeSelectors(nodes []string) (listOptions metav1.ListOptions) {
	listOptions = metav1.ListOptions{}
	listOptions.FieldSelector = combineWithPrefix(nodes, "metadata.name=", ",")
	return
}

func labelSelectors(selectors map[string]string) (listOptions metav1.ListOptions) {
	listOptions = metav1.ListOptions{}
	listOptions.LabelSelector = labels.SelectorFromSet(labels.Set(selectors)).String()
	return
}
