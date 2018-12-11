package main

import (
	"log"
	"math/rand"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func killNodes(testPlan ApplicationState, clientset *kubernetes.Clientset) {

	nodes := &v1.NodeList{}
	var err error
	// Attempt to get a list of all scheduleable nodes
	for true {
		listOptions := namedNodeSelectors(testPlan.Disruption.Nodes.Items)
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

		duration := time.Duration(rand.Int63n(testPlan.Disruption.Nodes.Interval.Nanoseconds())) * time.Nanosecond
		log.Printf("For next node cordon sleeping for %s\n", duration)
		time.Sleep(duration)
	}
}
