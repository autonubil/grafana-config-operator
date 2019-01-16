package cmd

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
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/kubernetes"

	"github.com/golang/glog"
	"gitlab.autonubil.net/kubernetes/grafana-config-operator/pkg/operator"
	"gitlab.autonubil.net/kubernetes/grafana-config-operator/pkg/utils"

	//	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func autoconfigure(options *operator.GrafanaConfigOperatorOptions) error {

	b, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace") // just pass the file name
	if err != nil {
		return err
	}
	namespace := string(b) // convert content to a 'string'
	glog.V(2).Infof("Pod Namespace is %s", namespace)

	// Create the client config for use in creating the k8s API client
	// Use kubeconfig if given, otherwise use in-clust
	kubecfg, err := utils.BuildKubeConfig(options.KubeConfig)
	if err != nil {
		return err
	}

	// Create a new k8s API client from the kubeconfig
	clientSet, err := kubernetes.NewForConfig(kubecfg)
	if err != nil {
		return err
	}

	podname, err := os.Hostname()
	if err != nil {
		return err
	}

	// debug
	// podname = "grafana-586857597c-6dlc2"

	glog.V(2).Infof("Pod Name is %s", podname)

	k8sOptions := meta_v1.GetOptions{IncludeUninitialized: false}
	pod, err := clientSet.Core().Pods(namespace).Get(podname, k8sOptions)
	if err != nil {
		return err
	}

	glog.V(11).Infof("Pod: %#v", pod)

	secretName := ""

	for _, container := range pod.Spec.Containers {
		if container.Name == "grafana" {
			for _, env := range container.Env {
				if env.Name == "GF_SECURITY_ADMIN_USER" {
					secretName = env.ValueFrom.SecretKeyRef.Name
					break
				}
			}
			break
		}

	}

	if secretName == "" {
		return errors.New("Faild to resolve secret name")
	}

	glog.V(2).Infof("Secret name: %s", secretName)
	secret, err := clientSet.Core().Secrets(namespace).Get(secretName, k8sOptions)
	if err != nil {
		return err
	}

	glog.V(11).Infof("Secret: %#v", secret)

	userRaw := secret.Data["admin-user"]
	if len(userRaw) == 0 {
		return errors.New("admin-user is empty")
	}
	passwordRaw := secret.Data["admin-password"]
	if len(passwordRaw) == 0 {
		return errors.New("admin-password is empty")
	}

	user := string(userRaw)
	/*
		make([]byte, base64.StdEncoding.DecodedLen(len(userRaw)))
		_, err = base64.StdEncoding.Decode(user, userRaw)
		if err != nil {
			return err
		}
	*/

	password := make([]byte, base64.StdEncoding.DecodedLen(len(passwordRaw)))
	_, err = base64.StdEncoding.Decode(password, passwordRaw)
	if err != nil {
		return err
	}

	options.GrafanaAuth = fmt.Sprintf("%s:%s", string(user), string(password))
	options.GrafanaEndpoint = "http://localhost:3000"
	glog.V(1).Infof("Succesfully autoconfigured")

	return nil

}

/*

func (p *HelmProvider) GetSecret(namespace string, name string) (interface{}, error) {
	options := meta_v1.GetOptions{IncludeUninitialized: false}
	secret, err := p.K8sClient.Core().Secrets(namespace).Get(name, options)
	if err != nil {
		log.Error("TemplateFunction.GetSecret", err, lager.Data{"namespace": namespace, "name": name})
		return nil, err
	}

	log.Debug("TemplateFunction.GetSecret", lager.Data{"namespace": namespace, "name": name, "Data": secret.Data})

	return secret.Data, nil
}

*/
