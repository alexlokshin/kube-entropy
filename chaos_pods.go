package main

import (
	"log"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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
