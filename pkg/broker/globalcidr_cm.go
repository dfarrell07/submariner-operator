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
package broker

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	GlobalCIDRConfigMapName = "submariner-globalnet-info"
	GlobalnetStatusKey      = "globalnetEnabled"
	ClusterInfoKey          = "clusterinfo"
	GlobalnetCidrRange      = "globalnetCidrRange"
	GlobalnetClusterSize    = "globalnetClusterSize"
)

type ClusterInfo struct {
	ClusterID  string   `json:"cluster_id"`
	GlobalCidr []string `json:"global_cidr"`
}

func CreateGlobalnetConfigMap(config *rest.Config, globalnetEnabled bool, defaultGlobalCidrRange string,
	defaultGlobalClusterSize uint, namespace string) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating the core kubernetes clientset: %s", err)
	}

	gnConfigMap, err := NewGlobalnetConfigMap(globalnetEnabled, defaultGlobalCidrRange, defaultGlobalClusterSize, namespace)
	if err != nil {
		return fmt.Errorf("error creating config map: %s", err)
	}

	_, err = clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), gnConfigMap, metav1.CreateOptions{})
	if err == nil || errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func NewGlobalnetConfigMap(globalnetEnabled bool, defaultGlobalCidrRange string,
	defaultGlobalClusterSize uint, namespace string) (*v1.ConfigMap, error) {
	labels := map[string]string{
		"component": "submariner-globalnet",
	}

	cidrRange, err := json.Marshal(defaultGlobalCidrRange)
	if err != nil {
		return nil, err
	}

	var data map[string]string
	if globalnetEnabled {
		data = map[string]string{
			GlobalnetStatusKey:   "true",
			GlobalnetCidrRange:   string(cidrRange),
			GlobalnetClusterSize: fmt.Sprint(defaultGlobalClusterSize),
			ClusterInfoKey:       "[]",
		}
	} else {
		data = map[string]string{
			GlobalnetStatusKey: "false",
			ClusterInfoKey:     "[]",
		}
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GlobalCIDRConfigMapName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
	return cm, nil
}

func UpdateGlobalnetConfigMap(k8sClientset *kubernetes.Clientset, namespace string,
	configMap *v1.ConfigMap, newCluster ClusterInfo) error {
	var clusterInfo []ClusterInfo
	err := json.Unmarshal([]byte(configMap.Data[ClusterInfoKey]), &clusterInfo)
	if err != nil {
		return err
	}

	exists := false
	for k, value := range clusterInfo {
		if value.ClusterID == newCluster.ClusterID {
			clusterInfo[k].GlobalCidr = newCluster.GlobalCidr
			exists = true
		}
	}

	if !exists {
		var newEntry ClusterInfo
		newEntry.ClusterID = newCluster.ClusterID
		newEntry.GlobalCidr = newCluster.GlobalCidr
		clusterInfo = append(clusterInfo, newEntry)
	}

	data, err := json.MarshalIndent(clusterInfo, "", "\t")
	if err != nil {
		return err
	}

	configMap.Data[ClusterInfoKey] = string(data)
	_, err = k8sClientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	return err
}

func GetGlobalnetConfigMap(k8sClientset *kubernetes.Clientset, namespace string) (*v1.ConfigMap, error) {
	cm, err := k8sClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), GlobalCIDRConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}
