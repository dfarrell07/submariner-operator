/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/submariner-io/submariner/pkg/cidr"
	smClientset "github.com/submariner-io/submariner/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/submariner-io/submariner-operator/apis/submariner/v1alpha1"
	"github.com/submariner-io/submariner-operator/pkg/internal/cli"
)

var validatePodsCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Check the Submariner deployment",
	Long:  "This command checks that the Submariner components are properly deployed and running with no overlapping CIDRs.",
	Run:   validateSubmarinerDeployment,
}

func init() {
	validateCmd.AddCommand(validatePodsCmd)
}

func validateSubmarinerDeployment(cmd *cobra.Command, args []string) {
	configs, err := getMultipleRestConfigs(kubeConfig, kubeContexts)
	exitOnError("Error getting REST config for cluster", err)

	validationStatus := true

	for _, item := range configs {
		status.Start(fmt.Sprintf("Retrieving Submariner resource from %q", item.clusterName))
		submariner := getSubmarinerResource(item.config)
		if submariner == nil {
			status.QueueWarningMessage(submMissingMessage)
			status.End(cli.Success)
			continue
		}

		status.End(cli.Success)

		validationStatus = validationStatus && checkPods(item, submariner, OperatorNamespace)
		validationStatus = validationStatus && checkOverlappingCIDRs(item, submariner)
	}

	if !validationStatus {
		os.Exit(1)
	}
}

func checkOverlappingCIDRs(item restConfig, submariner *v1alpha1.Submariner) bool {
	submarinerClient, err := smClientset.NewForConfig(item.config)
	exitOnError("Unable to get the Submariner client", err)

	if submariner.Spec.GlobalCIDR != "" {
		status.Start("Globalnet deployment detected, checking if globalnet CIDRs overlap")
	} else {
		status.Start("Non-Globalnet deployment detected, checking if cluster CIDRs overlap")
	}

	localClusterName := submariner.Status.ClusterID
	endpointList, err := submarinerClient.SubmarinerV1().Endpoints(submariner.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		message := fmt.Sprintf("Error listing the Submariner endpoints in cluster %q", localClusterName)
		status.QueueFailureMessage(message)
		status.End(cli.Failure)
		return false
	}

	var message string
	for i, source := range endpointList.Items {
		for _, dest := range endpointList.Items[i+1:] {
			// Currently we dont support multiple endpoints in a cluster, hence return an error.
			// When the corresponding support is added, this check needs to be updated.
			if source.Spec.ClusterID == dest.Spec.ClusterID {
				message = fmt.Sprintf("Found multiple Submariner endpoints (%q and %q) in cluster %q",
					source.Name, dest.Name, source.Spec.ClusterID)
				status.QueueFailureMessage(message)
				continue
			}

			for _, subnet := range dest.Spec.Subnets {
				overlap, err := cidr.IsOverlapping(source.Spec.Subnets, subnet)
				if err != nil {
					// Ideally this case will never hit, as the subnets are valid CIDRs
					message = fmt.Sprintf("Error parsing CIDR in cluster %q: %s", dest.Spec.ClusterID, err)
					status.QueueFailureMessage(message)
					continue
				}

				if overlap {
					message = fmt.Sprintf("CIDR %q in cluster %q overlaps with cluster %q (CIDRs: %v)",
						subnet, dest.Spec.ClusterID, source.Spec.ClusterID, source.Spec.Subnets)
					status.QueueFailureMessage(message)
				}
			}
		}
	}

	if status.HasFailureMessages() {
		status.End(cli.Failure)
		return false
	}

	if submariner.Spec.GlobalCIDR != "" {
		status.QueueSuccessMessage("Clusters do not have overlapping globalnet CIDRs")
	} else {
		status.QueueSuccessMessage("Clusters do not have overlapping CIDRs")
	}

	status.End(cli.Success)
	return true
}

func checkPods(item restConfig, submariner *v1alpha1.Submariner, operatorNamespace string) bool {
	message := fmt.Sprintf("Checking Submariner pods in %q", item.clusterName)
	status.Start(message)

	kubeClientSet, err := kubernetes.NewForConfig(item.config)

	if err != nil {
		exitOnError("Error creating Kubernetes client", err)
	}

	if !CheckDaemonset(kubeClientSet, operatorNamespace, "submariner-gateway") {
		return false
	}

	if !CheckDaemonset(kubeClientSet, operatorNamespace, "submariner-routeagent") {
		return false
	}

	// Check if service-discovery components are deployed and running if enabled
	if submariner.Spec.ServiceDiscoveryEnabled {
		// Check lighthouse-agent
		if !CheckDeployment(kubeClientSet, operatorNamespace, "submariner-lighthouse-agent") {
			return false
		}

		// Check lighthouse-coreDNS
		if !CheckDeployment(kubeClientSet, operatorNamespace, "submariner-lighthouse-coredns") {
			return false
		}
	}
	// Check if globalnet components are deployed and running if enabled
	if submariner.Spec.GlobalCIDR != "" {
		if !CheckDaemonset(kubeClientSet, operatorNamespace, "submariner-globalnet") {
			return false
		}
	}

	if !checkPodsStatus(kubeClientSet, operatorNamespace) {
		return false
	}

	message = "All Submariner pods are up and running"
	status.QueueSuccessMessage(message)
	status.End(cli.Success)
	return true
}

func CheckDeployment(k8sClient kubernetes.Interface, namespace, deploymentName string) bool {
	deployment, err := k8sClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		message := fmt.Sprintf("Error obtaining Deployment %q: %v", deploymentName, err)
		status.QueueFailureMessage(message)
		status.End(cli.Failure)
		return false
	}

	var replicas int32 = 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	if deployment.Status.AvailableReplicas != replicas {
		message := fmt.Sprintf("The desired number of replicas for Deployment %q (%d)"+
			" does not match the actual number running (%d)", deploymentName, replicas,
			deployment.Status.AvailableReplicas)
		status.QueueFailureMessage(message)
		status.End(cli.Failure)
		return false
	}

	return true
}

func CheckDaemonset(k8sClient kubernetes.Interface, namespace, daemonSetName string) bool {
	daemonSet, err := k8sClient.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err != nil {
		message := fmt.Sprintf("Error obtaining Daemonset %q: %v", daemonSetName, err)
		status.QueueFailureMessage(message)
		status.End(cli.Failure)
		return false
	}

	if daemonSet.Status.CurrentNumberScheduled != daemonSet.Status.DesiredNumberScheduled {
		message := fmt.Sprintf("The desired number of running pods for DaemonSet %q (%d)"+
			" does not match the actual number (%d)", daemonSetName, daemonSet.Status.DesiredNumberScheduled,
			daemonSet.Status.CurrentNumberScheduled)
		status.QueueFailureMessage(message)
		status.End(cli.Failure)
		return false
	}

	return true
}

func checkPodsStatus(k8sClient kubernetes.Interface, operatorNamespace string) bool {
	pods, err := k8sClient.CoreV1().Pods(operatorNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		message := fmt.Sprintf("Error obtaining Pods list: %v", err)
		status.QueueFailureMessage(message)
		status.End(cli.Failure)
		return false
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			message := fmt.Sprintf("Pod %q is not running. (current state is %v)", pod.Name, pod.Status.Phase)
			status.QueueFailureMessage(message)
			status.End(cli.Failure)
			return false
		}

		for _, c := range pod.Status.ContainerStatuses {
			if c.RestartCount >= 5 {
				message := fmt.Sprintf("Pod %q has restarted %d times", pod.Name, c.RestartCount)
				status.QueueWarningMessage(message)
			}
		}
	}

	return true
}
