package utils

/*
Copyright [2019] [autonubil System GmbH]

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

import (
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Build kubeconfig for use with clients.
// The kubeconfig file can either be passed in as a param, or attempted to be
// retrieved from the in-cluster ConfigMapAccount.
func BuildKubeConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		glog.V(2).Infof("kubeconfig file: %s", kubeconfig)
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	glog.V(2).Info("kubeconfig file: using InClusterConfig.")
	return rest.InClusterConfig()
}

func IsConfigMapLabeled(configMap *v1.ConfigMap, label string) bool {
	if _, exists := configMap.ObjectMeta.Labels[label]; exists {
		return true
	}
	return false
}
