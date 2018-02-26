# Mesitis Service Broker

Mesitis is a configuration-driven Service Broker for teams, built on using the Kubernetes Service Catalog. 

For more information,
[visit the Service Catalog project on github](https://github.com/kubernetes-incubator/service-catalog).

## Installing the Chart

Mesitis has several dependencies on and assumptions about resources that should exist in the cluster
and are not packaged in the Chart.

0. Namespaces

1. RBAC user, role, role bindings

2. ConfigMap for configuration

Mesitis depends extensively on ConfigMaps for configuration. That ConfigMap is not packaged in this chart. 
Instead a template and sample is available in the resources directory of the Mesitis repository.

After updating the ConfigMap to your liking, 

First, push an

To install the chart with the release name `mesitis`:

```bash
$ helm install charts/mesitis --name mesitis --namespace mesitis
```

## Uninstalling the Chart

To uninstall/delete the `mesitis` deployment:

```bash
$ helm delete mesitis
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The following tables lists the configurable parameters of the Mesitis
Service Broker

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image` | Image to use | `docker.io/jonahbenton/mesitis:v0.0.1` |
| `imagePullPolicy` | `imagePullPolicy` for the mesitis broker | `Always` |

Specify each parameter using the `--set key=value[,key=value]` argument to
`helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be
provided while installing the chart. For example:

```bash
$ helm install charts/mesitis --name mesitis --namespace mesitis \
  --values values.yaml
```
