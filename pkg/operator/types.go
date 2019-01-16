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

// Define a type for the options of grafanaConfigOperator
type GrafanaConfigOperatorOptions struct {
	KubeConfig        string
	Namespace         string
	Autoconfigure     bool
	PrometheusEnabled bool
	GrafanaEndpoint   string
	GrafanaAuth       string
	DatasourceWatch   bool
	DatasourceLabel   string
	DashboardWatch    bool
	DashboardLabel    string
	DbaasFolder       bool
}

func (opts *GrafanaConfigOperatorOptions) IsApiConfigured() bool {
	return (opts.GrafanaAuth != "" && opts.GrafanaEndpoint != "")
}
