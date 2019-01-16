package operator

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
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/kubernetes"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
	"gitlab.autonubil.net/kubernetes/grafana-config-operator/pkg/grafana"
	"gitlab.autonubil.net/kubernetes/grafana-config-operator/pkg/utils"
)

// Define a type for the options of grafanaConfigOperator
type GrafanaControllerOptions struct {
	KubeConfig      string
	Namespace       string
	GrafanaEndpoint string
	GrafanaAuth     string
	DashboardLabel  string
	DatasourceLabel string
	DbaasFolder     bool
}

// Implements an grafanaConfig's controller loop in a particular namespace.
// The controller makes use of an Informer resource to locally cache resources
// managed, and handle events on the resources.
type grafanaConfigController struct {
	// Baseline kubeconfig to use when communicating with the API.
	kubecfg *rest.Config

	// Clientset that has a REST client for each k8s API group.
	clientSet kubernetes.Interface

	// Informer for all resources being watched by the operator.
	informer *grafanaConfigControllerInformer

	// The namespace where the operator is running.
	namespace string

	options *GrafanaControllerOptions
}

// Implements an Informer for the resources being operated on: ConfigMaps &
// grafanaConfigs.
type grafanaConfigControllerInformer struct {
	// Store & controller for ConfigMap resources
	configmapStore      cache.Store
	configmapController cache.Controller
}

// Create a new Controller for the grafanaConfig operator
func NewgrafanaConfigController(options *GrafanaControllerOptions) (
	*grafanaConfigController, error) {

	// Create the client config for use in creating the k8s API client
	// Use kubeconfig if given, otherwise use in-clust
	kubecfg, err := utils.BuildKubeConfig(options.KubeConfig)
	if err != nil {
		return nil, err
	}

	// Create a new k8s API client from the kubeconfig
	clientSet, err := kubernetes.NewForConfig(kubecfg)
	if err != nil {
		return nil, err
	}

	// Create a new k8s REST API client for grafanaConfigs
	// Create new grafanaConfigController
	npc := &grafanaConfigController{
		kubecfg:   kubecfg,
		clientSet: clientSet,
		options:   options,
	}

	// Create a new Informer for the grafanaConfigController
	npc.informer = npc.newGrafanaConfigControllerInformer()

	return npc, nil
}

// Start the grafanaConfigController until stopped.
func (npc *grafanaConfigController) Start(stop <-chan struct{}) {
	// Don't let panics crash the process
	defer utilruntime.HandleCrash()

	npc.start(stop)

	// Block until stopped
	<-stop
}

// Start the controllers with the stop chan as required by Informers.
func (npc *grafanaConfigController) start(stop <-chan struct{}) {
	namespace := npc.namespace
	if namespace == "" {
		namespace = "<any>"
	}

	watched := ""
	if npc.options.DashboardLabel != "" {
		watched = watched + npc.options.DashboardLabel
	}
	if npc.options.DatasourceLabel != "" {
		if watched != "" {
			watched = watched + " and "
		}
		watched = watched + npc.options.DatasourceLabel
	}
	glog.V(2).Infof("Start watching Namespace: %s for %s", namespace, watched)

	// Run controller for ConfigMap Informer and handle events via callbacks
	go npc.informer.configmapController.Run(stop)

}

// Informers are a combination of a local cache store to buffer the state of a
// given resource locally, and a controller to handle events through callbacks.
//
// Informers sync the APIServer's state of a resource with the local cache
// store.

// Creates a new Informer for the grafanaConfigController.
// An grafanaConfigController uses a set of Informers to watch and operate on
// ConfigMaps and grafanaConfig resources in its control loop.
func (npc *grafanaConfigController) newGrafanaConfigControllerInformer() *grafanaConfigControllerInformer {
	configMapStore, configMapController := npc.newConfigMapInformer()

	return &grafanaConfigControllerInformer{
		configmapStore:      configMapStore,
		configmapController: configMapController,
	}
}

// Create a new Informer on the ConfigMap resources in the cluster to track them.
func (npc *grafanaConfigController) newConfigMapInformer() (cache.Store, cache.Controller) {
	var timeout int64
	timeout = 30
	filter := ""
	if (npc.options.DashboardLabel != "") && (npc.options.DatasourceLabel == "") {
		filter = npc.options.DashboardLabel
	} else if (npc.options.DatasourceLabel != "") && (npc.options.DatasourceLabel == "") {
		filter = npc.options.DatasourceLabel
	}

	return cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(alo metav1.ListOptions) (runtime.Object, error) {
				// Retrieve a ConfigMapList from the the API
				lo := metav1.ListOptions{IncludeUninitialized: false, TimeoutSeconds: &timeout}
				if filter != "" {
					lo.LabelSelector = filter
				}
				return npc.clientSet.CoreV1().ConfigMaps(npc.namespace).List(lo)
			},
			WatchFunc: func(alo metav1.ListOptions) (watch.Interface, error) {
				// Watch the ConfigMaps in the API
				lo := metav1.ListOptions{IncludeUninitialized: false}
				if filter != "" {
					lo.LabelSelector = filter
				}
				return npc.clientSet.CoreV1().ConfigMaps(npc.namespace).Watch(lo)
			},
		},
		// The resource that the informer returns
		&corev1.ConfigMap{},
		// The sync interval of the informer
		0*time.Second,
		// Callback functions for add, delete & update events
		cache.ResourceEventHandlerFuncs{
			// AddFunc: func(o interface{}) {}
			UpdateFunc: npc.handleConfigMapUpdate,
			DeleteFunc: npc.handleConfigMapDelete,
		},
	)
}

func (npc *grafanaConfigController) isWatchedLabel(configMap *corev1.ConfigMap) bool {
	if (npc.options.DashboardLabel != "") && utils.IsConfigMapLabeled(configMap, npc.options.DashboardLabel) {
		return true
	}
	if (npc.options.DatasourceLabel != "") && utils.IsConfigMapLabeled(configMap, npc.options.DatasourceLabel) {
		return true
	}
	return false
}

func (npc *grafanaConfigController) handleConfigMapDelete(obj interface{}) {
	configMap := obj.(*corev1.ConfigMap)
	glog.V(11).Infof("Received delete for ConfigMap: %s/%s", configMap.Namespace, configMap.Name)
	if npc.isWatchedLabel(configMap) {
		npc.processConfigMap(configMap, true)
	} else {
		glog.V(12).Infof("Skipping non grafana labeled ConfigMap: %s/%s", configMap.Namespace, configMap.Name)
	}
}

// Callback for updates to a ConfigMap Informer
func (npc *grafanaConfigController) handleConfigMapUpdate(oldObj, newObj interface{}) {
	configMap := newObj.(*corev1.ConfigMap)
	// oldConfigMap := oldObj.(*corev1.ConfigMap)
	glog.V(11).Infof("Received update for ConfigMap: %s/%s", configMap.Namespace, configMap.Name)
	if npc.isWatchedLabel(configMap) {
		npc.processConfigMap(configMap, false)
	} else {
		glog.V(12).Infof("Skipping non grafana labeled ConfigMap: %s/%s", configMap.Namespace, configMap.Name)
	}
}

func (npc *grafanaConfigController) processConfigMap(configMap *corev1.ConfigMap, deleteMode bool) {
	glog.V(3).Infof("Processing Config Map: %s/%s", configMap.Namespace, configMap.Name)
	for file, content := range configMap.Data {
		// yaml or json? DataSource or Dashboard?
		ds, board, err := grafana.GetGrafanaConfigObjectFromString(content)
		if err != nil {
			glog.Errorf("Failed to unmarshall grafana configuration object from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
			raven.CaptureError(err, map[string]string{"operation": "GetGrafanaConfigObjectFromString", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			continue
		}

		if ds != nil {
			if npc.options.DatasourceLabel == "" {
				glog.Errorf("Datasource found, but DatasourceLabel not confgured. Config Map: %s/%s %s", configMap.Namespace, configMap.Name, file)
				continue
			} else if !utils.IsConfigMapLabeled(configMap, npc.options.DatasourceLabel) {
				glog.Errorf("Datasource found, but not with configured label  (%s). Config Map: %s/%s %s", npc.options.DatasourceLabel, configMap.Namespace, configMap.Name, file)
				continue
			} else {
				if ds.ApiVersion != 1 {
					glog.Errorf("Unsupported API Version %d Config Map: %s/%s %s", ds.ApiVersion, configMap.Namespace, configMap.Name, file)
					continue
				}
				npc.processDatasourceConfigMap(configMap, file, ds, deleteMode)
			}
		}
		if board != nil {
			if npc.options.DashboardLabel == "" {
				glog.Errorf("Dashboard found, but DashboardLabel not confgured. Config Map: %s/%s %s", configMap.Namespace, configMap.Name, file)
				continue
			} else if !utils.IsConfigMapLabeled(configMap, npc.options.DashboardLabel) {
				glog.Errorf("Dashboard found, but as with configured label (%s). Config Map: %s/%s %s", npc.options.DashboardLabel, configMap.Namespace, configMap.Name, file)
				continue
			} else {
				if deleteMode {
					npc.deleteDashboardConfigMap(configMap, file, board)
				} else {
					npc.processDashboardConfigMap(configMap, file, board)
				}
			}
		}

	}
}

func (npc *grafanaConfigController) processDatasourceConfigMap(configMap *corev1.ConfigMap, file string, config *grafana.DatasourceConfigFile, deleteMode bool) {
	if deleteMode {
		glog.V(2).Infof("Handling Delete Datasource Config Map in namespace %s/%s", configMap.Namespace, configMap.Name)
	} else {
		glog.V(2).Infof("Handling Update Datasource Config Map: %s/%s", configMap.Namespace, configMap.Name)
	}
	// via API
	grafanaClient := grafana.NewClient(npc.options.GrafanaEndpoint, npc.options.GrafanaAuth, grafana.DefaultHTTPClient)
	for _, datasourceToDelete := range config.DeleteDatasources {
		existingDs, err := grafanaClient.GetDatasourceByName(datasourceToDelete.Name)
		if err != nil {
			glog.V(4).Infof("Datasource %s from Config Map: %s/%s %s does not exist info ", datasourceToDelete.Name, configMap.Namespace, configMap.Name, file)
		} else {
			_, err := grafanaClient.DeleteDatasource(existingDs.ID)
			if err != nil {
				glog.Errorf("Failed to unmarshall datasource info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
				raven.CaptureError(err, map[string]string{"operation": "DeleteDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToDelete.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			} else {
				glog.V(1).Infof("Deleted Datasource %s from Config Map: %s/%s %s", datasourceToDelete.Name, configMap.Namespace, configMap.Name, file)
				raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Deleted Data Source"}, map[string]string{"operation": "DeleteDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToDelete.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			}
		}
	}

	for _, datasourceToEnsure := range config.Datasources {
		existingDs, err := grafanaClient.GetDatasourceByName(datasourceToEnsure.Name)
		if err != nil && err.Error() != "HTTP error 404: returns {\"message\":\"Data source not found\"}" {
			glog.Errorf("Failed to check for existing datasource info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
			raven.CaptureError(err, map[string]string{"operation": "GetDatasourceByName", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			continue
		} else if existingDs.ID != 0 {
			if deleteMode {
				_, err := grafanaClient.DeleteDatasource(existingDs.ID)
				if err != nil {
					glog.Errorf("Failed to unmarshall datasource info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
					raven.CaptureError(err, map[string]string{"operation": "DeleteDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
				} else {
					glog.V(1).Infof("Deleted Datasource %s from Config Map: %s/%s %s", datasourceToEnsure.Name, configMap.Namespace, configMap.Name, file)
					raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Deleted Data Source"}, map[string]string{"operation": "DeleteDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
				}
			} else {
				glog.V(3).Infof("Datasource %s from Config Map: %s/%s %s already exists with id %d. Will Update....", datasourceToEnsure.Name, configMap.Namespace, configMap.Name, file, existingDs.ID)
				datasourceToEnsure.ID = existingDs.ID
				_, err = grafanaClient.UpdateDatasource(datasourceToEnsure)
				if err != nil {
					glog.Errorf("Failed to Update datasource info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
					raven.CaptureError(err, map[string]string{"operation": "CreateDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
				} else {
					glog.V(1).Infof("Updated Datasource %s from Config Map: %s/%s %s", datasourceToEnsure.Name, configMap.Namespace, configMap.Name, file)
					raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Created Data Source"}, map[string]string{"operation": "CreateDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
				}
			}
		} else {
			_, err = grafanaClient.CreateDatasource(datasourceToEnsure)
			if err != nil {
				glog.Errorf("Failed to create datasource info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
				raven.CaptureError(err, map[string]string{"operation": "CreateDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			} else {
				glog.V(1).Infof("Created Datasource %s from Config Map: %s/%s %s", datasourceToEnsure.Name, configMap.Namespace, configMap.Name, file)
				raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Created Data Source"}, map[string]string{"operation": "CreateDatasource", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "DataSource.Name": datasourceToEnsure.Name, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			}
		}
	}
}

func (npc *grafanaConfigController) processDashboardConfigMap(configMap *corev1.ConfigMap, file string, board *grafana.Board) {
	glog.V(2).Infof("Handling Update Dashboard %s from Config Map: %s/%s", file, configMap.Namespace, configMap.Name)

	grafanaClient := grafana.NewClient(npc.options.GrafanaEndpoint, npc.options.GrafanaAuth, grafana.DefaultHTTPClient)

	folderID := uint(0) // General

	// is the board in a subfolder
	path := strings.Split(file, ".")
	if strings.ToLower(path[len(path)-1]) == "json" || strings.ToLower(path[len(path)-1]) == "js" {
		path = path[:len(path)-1]
	}
	path = path[:len(path)-1]
	if len(path) > 0 {
		targetFolder, err := grafanaClient.GetFolderByTitle(path[0])
		if err != nil {
			glog.Errorf("Failed to check list folders (%#v)", err)
			raven.CaptureError(err, map[string]string{"operation": "GetAllFolders", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			return
		}
		if targetFolder == nil {
			statusMessage, err := grafanaClient.CreateFolder(grafana.Folder{Title: path[0]})
			if err != nil {
				glog.Errorf("Failed to create folder (%#v)", err)
				raven.CaptureError(err, map[string]string{"operation": "CreateFolder", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
				return
			}
			raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Created new Folder"}, map[string]string{"operation": "CreateFolder", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "GrafanaEndpoint": npc.options.GrafanaEndpoint, "Folder.Name": path[0]})
			folderID = *statusMessage.ID
		} else {
			folderID = targetFolder.ID
		}
	}

	err := grafanaClient.SetDashboard(*board, true, folderID)
	if err != nil {
		glog.Errorf("Failed to check for existing dashboard info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
		raven.CaptureError(err, map[string]string{"operation": "GetDashboardByName", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
		return
	}

	glog.V(1).Infof("Created or Updated Dashboard %s from Config Map: %s/%s %s", board.Title, configMap.Namespace, configMap.Name, file)
	raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Created Dashboard"}, map[string]string{"operation": "CreateDashboard", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
}

func (npc *grafanaConfigController) deleteDashboardConfigMap(configMap *corev1.ConfigMap, file string, board *grafana.Board) {
	glog.V(2).Infof("Handling Delete Dashboard %s from Config Map: %s/%s %s", file, configMap.Namespace, configMap.Name)

	grafanaClient := grafana.NewClient(npc.options.GrafanaEndpoint, npc.options.GrafanaAuth, grafana.DefaultHTTPClient)
	if board.UID != "" {
		_, _, err := grafanaClient.GetDashboard(board.UID)
		if err != nil {
			glog.Errorf("Failed to check for existing dashboard info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
			raven.CaptureError(err, map[string]string{"operation": "GetDashboardBy", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "Board.UID": board.UID, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			return
		}
		_, err = grafanaClient.DeleteDashboard(board.UID)
		if err != nil {
			glog.Errorf("Failed to Delete  existing dashboard info from Config Map: %s/%s %s (%#v)", configMap.Namespace, configMap.Name, file, err)
			raven.CaptureError(err, map[string]string{"operation": "DeleteDashboard", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "Board.UID": board.UID, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
			return
		}
		raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Delete Dashboard"}, map[string]string{"operation": "DeleteDashboard", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
		return
	}
	raven.Capture(&raven.Packet{Level: raven.WARNING, Message: "Canont delete Dashboard without UID"}, map[string]string{"operation": "deleteDashboardConfigMap", "ConfigMap.Namespace": configMap.Namespace, "CopnfigMap.Name": configMap.Name, "ConfigMap.File": file, "Board.Name": board.Title, "GrafanaEndpoint": npc.options.GrafanaEndpoint})
}
