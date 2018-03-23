package controller

import (
	"errors"
	"fmt"
	"sort"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"

	"github.com/jonahbenton/mesitis/pkg/chartdl"
)

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func (instance *Instance) Deprovision(kube Kube) error {
	var err error
	if instance.ResourcesKubeObjectList != nil {
		for i := len(*(*instance).ResourcesKubeObjectList) - 1; i >= 0; i-- {
			po := (*(*instance).ResourcesKubeObjectList)[i]
			switch po.Kind {
			case "pod":
				err = kube.DeletePod(po.Namespace, po.Name)
			case "deployment":
				err = kube.DeleteDeployment(po.Namespace, po.Name)
			case "service":
				err = kube.DeleteService(po.Namespace, po.Name)
			case "configmap":
				err = kube.DeleteConfigMap(po.Namespace, po.Name)
			case "secret":
				err = kube.DeleteSecret(po.Namespace, po.Name)
			default:
				glog.Errorf("Unable to delete provisioned object kind: %s", po.Kind)
			}
			if err != nil {
				glog.Errorf("Failed to delete provisioned object: %s", err)
			}
		}

	} else if instance.ResourcesNoResource != nil {
		// nothing to do
	} else if instance.ResourcesHelmRelease != nil {

		// tillerHost := "tiller-deploy.kube-system.svc.cluster.local:44134"
		// helmClient := helm.NewClient(helm.Host(tillerHost))
		// if _, err := helmClient.DeleteRelease(instance.ResourcesHelmRelease.Name); err != nil {
		// return err
		// }
	}
	return nil
}

type Provision interface {
	Provision(kube Kube, id string, entry *Entry) (*Instance, error)
}

func (e *Entry) Provision(kube Kube, id string) (*Instance, error) {
	if e.ProvisionExistingClusterService != nil {
		return e.ProvisionExistingClusterService.Provision(kube, id, e)
	} else if e.ProvisionNonClusterURL != nil {
		return e.ProvisionNonClusterURL.Provision(kube, id, e)
	} else if e.ProvisionNewClusterObjects != nil {
		return e.ProvisionNewClusterObjects.Provision(kube, id, e)
	} else if e.ProvisionHelmChart != nil {
		return e.ProvisionHelmChart.Provision(kube, id, e)
	} else {
		glog.Errorln("Missing provision type")
		return nil, errors.New("Failed to provision")
	}
}

func (p ProvisionExistingClusterService) Provision(kube Kube, id string, entry *Entry) (*Instance, error) {
	URL := fmt.Sprintf("%s.%s.svc.cluster.local", p.Name, p.Namespace)

	instance := Instance{*entry, id, nil, &CoordinatesClusterURL{URL: URL}, &ResourcesNoResource{}, nil, nil}
	return &instance, nil
}

func (p ProvisionNonClusterURL) Provision(kube Kube, id string, entry *Entry) (*Instance, error) {
	URL := p.URL
	instance := Instance{*entry, id, &CoordinatesExternalURL{URL: URL}, nil, &ResourcesNoResource{}, nil, nil}
	return &instance, nil
}

func (p ProvisionHelmChart) Provision(kube Kube, id string, entry *Entry) (*Instance, error) {

	chartURL := p.ChartURL
	// TODO consider adding tillerHost to kube object, getting it from app configuration
	// tillerHost := "tiller-deploy.kube-system.svc.cluster.local:44134"

	// TODO consider checking whether the release name exists in the namespace already first

	destdir := kube.(*RealKube).Tmpdir
	tarroot, err := chartdl.DownloadChart(destdir, chartURL)
	if err == nil {
		glog.Infof("Chart at URL <%s> downloaded to <%s>", chartURL, tarroot)
	} else {
		glog.Infof("Chart at URL <%s> download failed <%s>", chartURL, err)
	}
	// TODO install chart in p.Namespace
	// 	vals, err := yaml.Marshal(map[string]interface{}{
	//	"mariadbRootPassword": uniuri.New(),
	//  "mariadbDatabase":     "dbname",
	// })
	// 	helmClient := helm.NewClient(helm.Host(tillerHost))
	// _, err = helmClient.InstallRelease(tarroot, p.Namespace, helm.ReleaseName(name), helm.ValueOverrides(vals))

	URL := fmt.Sprintf("%s.%s.svc.cluster.local", p.Name, p.Namespace)
	instance := Instance{*entry, id, &CoordinatesExternalURL{URL: URL}, nil, nil, nil, &ResourcesHelmRelease{Namespace: p.Namespace, Name: p.Name}}
	return &instance, nil
}

// TODO rename to InOrder
type ByOrder []v1.ConfigMap

func (a ByOrder) Len() int      { return len(a) }
func (a ByOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool {
	return a[i].ObjectMeta.Labels["mesitis/order"] < a[j].ObjectMeta.Labels["mesitis/order"]
}

func (p ProvisionNewClusterObjects) Provision(kube Kube, id string, entry *Entry) (*Instance, error) {

	// TODO consider checking whether a service with the given name exists in the namespace

	const dataKey = "embedded-resource"

	obj := p

	glog.Infof("Attempting to find config maps matching labelselector: %s\n", obj.LabelSelector)
	list, err := kube.ListConfigMaps(kube.BrokerNamespace(), obj.LabelSelector)
	if err != nil {
		// TODO is this an error, or provision anyway?
		glog.Errorf("Failed to find config maps from which to provision object: %s\n", err)
		return nil, err
	}

	// TODO rename pcfo, no longer relevant
	pcfo := ResourcesKubeObjectList{}

	// ensure objects created in their specified order
	sort.Sort(ByOrder(list.Items))
	for _, cm := range list.Items {
		if cm.ObjectMeta.Labels["mesitis/enabled"] != "true" {
			continue
		}
		// TODO this should not be a switch but instead a map
		// behavior is identical
		// TODO check if the object exists already in the namespace. if so, don't provision again.
		switch cm.ObjectMeta.Labels["mesitis/kind"] {
		case "pod":
			pod, cerr := kube.CreatePodFromJSON(obj.Namespace, cm.Data[dataKey])
			if cerr == nil {
				glog.Infof("Created pod: %s\n", pod)
				pcfo = append(pcfo,
					ResourcesKubeObject{
						Kind:      "pod",
						Name:      pod.ObjectMeta.Name,
						Namespace: obj.Namespace})

				glog.Infof("Resources: %s\n", pcfo)
			} else {
				kubeError(cerr)
			}
		case "deployment":
			deployment, cerr := kube.CreateDeploymentFromJSON(obj.Namespace, cm.Data[dataKey])
			if cerr == nil {
				glog.Infof("Created deployment: %s\n", deployment)
				pcfo = append(pcfo,
					ResourcesKubeObject{
						Kind:      "deployment",
						Name:      deployment.ObjectMeta.Name,
						Namespace: obj.Namespace})
				glog.Infof("Resources: %s\n", pcfo)
			} else {
				kubeError(cerr)
			}

		case "service":
			service, cerr := kube.CreateServiceFromJSON(obj.Namespace, cm.Data[dataKey])
			if cerr == nil {
				glog.Errorf("Created service: %s\n", service)
				pcfo = append(pcfo,
					ResourcesKubeObject{
						Kind:      "service",
						Name:      service.ObjectMeta.Name,
						Namespace: obj.Namespace})
				glog.Infof("Resources: %s\n", pcfo)
			} else {
				kubeError(cerr)
			}

		case "configmap":
			configMap, cerr := kube.CreateConfigMapFromJSON(obj.Namespace, cm.Data[dataKey])
			if cerr == nil {
				glog.Errorf("Created ConfigMap: %s\n", configMap)
				pcfo = append(pcfo,
					ResourcesKubeObject{
						Kind:      "configmap",
						Name:      configMap.ObjectMeta.Name,
						Namespace: obj.Namespace})
				glog.Infof("Resources: %s\n", pcfo)
			} else {
				kubeError(cerr)
			}

		case "secret":
			secret, cerr := kube.CreateSecretFromJSON(obj.Namespace, cm.Data[dataKey])
			if cerr == nil {
				glog.Errorf("Created secret: %s\n", secret)
				pcfo = append(pcfo,
					ResourcesKubeObject{
						Kind:      "secret",
						Name:      secret.ObjectMeta.Name,
						Namespace: obj.Namespace})
				glog.Infof("Resources: %s\n", pcfo)
			} else {
				kubeError(cerr)
			}

		default:
			glog.Errorf("Don't know how to create object: %s", cm.ObjectMeta.Labels["mesitis/kind"])
		}
	}

	URL := fmt.Sprintf("%s.%s.svc.cluster.local", p.Name, p.Namespace)
	instance := Instance{*entry, id, nil, &CoordinatesClusterURL{URL: URL}, nil, &pcfo, nil}

	return &instance, nil
}

func (c *Entry) serviceName() string {
	return fmt.Sprintf("%s-%s", c.Team, c.Offering)
}

func (c *Entry) planName() string {
	return fmt.Sprintf("%s-%s-%s", c.Team, c.Offering, c.Version)
}

func (c *Entry) planDescription() string {
	return fmt.Sprintf("Version %s", c.Version)
}
