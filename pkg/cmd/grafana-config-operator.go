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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	goflag "flag"

	raven "github.com/getsentry/raven-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"gitlab.autonubil.net/kubernetes/grafana-config-operator/pkg/operator"
)

var (
	cmdName = "grafana-config-operator"
	license = "Copyright [2019] [autonubil System GmbH]\n" +
		"\n" +
		"Licensed under the Apache License, Version 2.0 (the \"License\");\n" +
		"you may not use this file except in compliance with the License.\n" +
		"You may obtain a copy of the License at\n" +
		"\n" +
		"	http://www.apache.org/licenses/LICENSE-2.0\n" +
		"\n" +
		"Unless required by applicable law or agreed to in writing, software\n" +
		"distributed under the License is distributed on an \"AS IS\" BASIS,\n" +
		"WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.\n" +
		"See the License for the specific language governing permissions and\n" +
		"limitations under the License."

	usage = fmt.Sprintf("%s\n%s", cmdName, license)
)

// Fatal prints the message (if provided) and then exits. If V(2) or greater,
// glog.Fatal is invoked for extended information.
func fatal(msg string) {
	if glog.V(2) {
		glog.FatalDepth(2, msg)
	}
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
	}
	os.Exit(1)
}

// NewCmdOptions creates an options Cobra command to return usage
func NewCmdOptions() *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}

	return cmd
}

// Create a new command for the grafanaConfig-operator. This cmd includes logging,
// cmd option parsing from flags, and the customization of the Tectonic assets.
func NewCmdGrafanaConfigOperator() (*cobra.Command, error) {
	// Define the options for grafanaConfigOperator command
	options := operator.GrafanaConfigOperatorOptions{
		PrometheusEnabled: true,
		GrafanaEndpoint:   "",
		DashboardWatch:    true,
		DashboardLabel:    "grafana_dashboard",
		DatasourceWatch:   true,
		DatasourceLabel:   "grafana_datasource",
		Autoconfigure:     false,
		DbaasFolder:       false,
	}

	// Create a new command
	cmd := &cobra.Command{
		Use:   usage,
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			checkErr(Run(cmd, &options), fatal)
		},
	}

	// Bind & parse flags defined by external projects.
	// e.g. This imports the golang/glog pkg flags into the cmd flagset
	cmd.Flags().AddGoFlagSet(goflag.CommandLine)
	goflag.CommandLine.Parse([]string{})

	cmd.Flags().StringVarP(&options.KubeConfig, "kubeconfig", "", options.KubeConfig, "Path to a kube config. Only required if out-of-cluster.")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", options.Namespace, "Namespace to watch for annotated configMaps in. If no namespace is provided, NAMESPACE env. var is used. Lastly, the '' (any namespaces) will be used as a last option.")
	cmd.Flags().BoolVarP(&options.PrometheusEnabled, "prometheus", "p", options.PrometheusEnabled, "Enable Prometheus metrics on port 9350. If not specified PROMETHEUS_ENABLES env. var is checked for existence")

	cmd.Flags().StringVarP(&options.GrafanaEndpoint, "grafana.endpoint", "e", options.GrafanaEndpoint, "Api Endpoint for grafana.")
	cmd.Flags().StringVarP(&options.GrafanaAuth, "grafana.auth", "t", options.GrafanaAuth, "grafana authentication (wheter basic <user:password> or <token>).")

	cmd.Flags().BoolVarP(&options.DatasourceWatch, "datasources.watch", "x", options.DatasourceWatch, "Watch for datasources")
	cmd.Flags().StringVarP(&options.DatasourceLabel, "datasources.label", "d", options.DatasourceLabel, "watch configmaps")

	cmd.Flags().BoolVarP(&options.DashboardWatch, "dashboards.watch", "w", options.DashboardWatch, "Watch for dashboards")
	cmd.Flags().StringVarP(&options.DashboardLabel, "dashboards.label", "l", options.DashboardLabel, "config map filter label. If ot specified, DASHBOARD_LABEL  env. var is checked for existence")

	cmd.Flags().BoolVarP(&options.DbaasFolder, "dbaasFolder", "z", options.DbaasFolder, "Create Folder for dashboards from the namespaces 'customergroup' label")

	return cmd, nil
}

func serveMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":9350", nil)
}

// Run the customization of the Tectonic assets
func Run(cmd *cobra.Command, options *operator.GrafanaConfigOperatorOptions) error {

	// Get Unsert Options from Environment
	if options.Namespace == "" {
		if options.Namespace = os.Getenv("NAMESPACE"); options.Namespace == "" {
			options.Namespace = ""
		}
	}

	if options.DashboardLabel == "" {
		if options.DashboardLabel = os.Getenv("DASHBOARD_LABEL"); options.DashboardLabel == "" {
			options.DashboardLabel = ""
		}
	}

	if options.DatasourceLabel == "" {
		if options.DatasourceLabel = os.Getenv("DATASOURCE_LABEL"); options.DatasourceLabel == "" {
			options.DatasourceLabel = ""
		}
	}

	configTags := make(map[string]string)
	if options.Namespace != "" {
		configTags["Namespace"] = options.Namespace
	}
	if options.GrafanaEndpoint != "" {
		configTags["GrafanaEndpoint"] = options.GrafanaEndpoint
	}
	if options.PrometheusEnabled {
		configTags["PrometheusEnabled"] = "true"
	}

	raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Started grafana Portmap Operator"}, configTags)

	if options.PrometheusEnabled {
		go serveMetrics()
	}

	if options.Autoconfigure {
		err := autoconfigure(options)
		if err != nil {
			glog.Errorf("Failed to autoconfigure Grafana parameters %#v", err)
		}
	}

	// Relay OS signals to the chan
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})

	opts := &operator.GrafanaControllerOptions{
		KubeConfig: options.KubeConfig,
		Namespace:  options.Namespace,

		GrafanaEndpoint: options.GrafanaEndpoint,
		GrafanaAuth:     options.GrafanaAuth,
		DbaasFolder:     options.DbaasFolder,
	}

	if options.DashboardWatch {
		opts.DashboardLabel = options.DashboardLabel
	}
	if options.DatasourceWatch {
		opts.DatasourceLabel = options.DatasourceLabel
	}
	if options.DatasourceWatch || options.DashboardWatch {
		watchCntlr, err := operator.NewgrafanaConfigController(opts)

		if err != nil {
			return err
		}
		go watchCntlr.Start(stop)
	}

	// Block until signaled to stop
	<-signals

	// Close the stop chan / shutdown the controller
	close(stop)
	glog.Infof("Shutting down grafanaConfigOperator...")
	raven.Capture(&raven.Packet{Level: raven.INFO, Message: "Stopped grafana Portmap Operator..."}, map[string]string{})

	return nil
}

func checkErr(err error, handleErr func(string)) {
	if err == nil {
		return
	}

	raven.CaptureError(err, map[string]string{"operation": "checkErr"})

	handleErr(err.Error())
}
