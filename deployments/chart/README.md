# Grafana Config Operator

Use config maps as Dashboard or datasource configuration for grafana


## Installing the Chart

To install the chart with the release name `grafana-config-operator`:

```bash
helm install deployments/chart --name grafana-config-operator --namespace grafana-config-operator
```

## Uninstalling the Chart

To uninstall/delete the `grafana-config-operator` deployment:

```bash
helm delete grafana-config-operator
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The following tables lists the configurable parameters of the User Provided
Service Broker

| Parameter | Description | Default |
| --------- | ----------- | ------- |

Specify each parameter using the `--set key=value[,key=value]` argument to
`helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be
provided while installing the chart. For example:

```bash
$ helm install deployments/chart --name grafana-config-operator --namespace grafana-config-operator \
  --values values.yaml
``
