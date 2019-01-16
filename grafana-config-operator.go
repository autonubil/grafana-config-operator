package main

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
	"os"

	k8slogsutil "k8s.io/apiserver/pkg/util/logs"

	"gitlab.autonubil.net/kubernetes/grafana-config-operator/pkg/cmd"

	"github.com/getsentry/raven-go"
	"github.com/golang/glog"
)

var Version string
var Commit string
var BuildDate string

func main() {

	sentryDsn := os.Getenv("SENTRY_DSN")
	if len(sentryDsn) > 0 {
		raven.SetDSN(sentryDsn)
		raven.SetRelease(fmt.Sprintf("%s [%s@%s]", Version, Commit, BuildDate))
		// Make sure that the call to doStuff doesn't leak a panic
		raven.CapturePanic(run, nil)
	} else {
		run()
	}

}

func run() {
	// Create & execute new command
	cmd, err := cmd.NewCmdGrafanaConfigOperator()
	if err != nil {
		os.Exit(1)
	}

	// Init logging
	k8slogsutil.InitLogs()
	defer k8slogsutil.FlushLogs()

	glog.Infof("Starting Grafana Config Operator [Version %s, Commit: %s, BuildDate: %s]", Version, Commit, BuildDate)

	err = cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
