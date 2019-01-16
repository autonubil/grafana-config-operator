This operator replaces the sidecar solution for dashboard and datasource
synchronization in the stable Grafana Chart. It only has two improvements over
the original solution:
 - Create Datasources via API without requiring a grafana restart
 - Allow Dashboards to be placed into folders

The Grafana API Client ist based on the work of Alexander I.Grafov.



See:
 - https://github.com/helm/charts/tree/master/stable/grafana
 - https://github.com/grafana-tools/sdk

# Usage

deploy using the helm chart in deployments/chart
```
$ helm install  --name gco --set config.grafana.endpoint="http://grafana:3000" --set config.grafana.auth="admin:admin123" ./deployments/chart
```
