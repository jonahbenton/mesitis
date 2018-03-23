package controller

import (
	"errors"
	"fmt"
	"sort"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"

	"github.com/jonahbenton/mesitis/pkg/chartdl"
)

type Provision interface {
	Provision(kube Kube, namespace string, id string) (*Instance, error)
}

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
	}
	return nil
}

func (e *Entry) Provision(kube Kube, namespace string, id string) (*Instance, error) {
	if e.ProvisionExistingClusterService != nil {
		return e.ProvisionExistingClusterService.Provision(kube, namespace, id)
	} else if e.ProvisionNonClusterURL != nil {
		return e.ProvisionNonClusterURL.Provision(kube, namespace, id)
	} else if e.ProvisionNewClusterObjects != nil {
		return e.ProvisionNewClusterObjects.Provision(kube, namespace, id)
	} else if e.ProvisionHelmChart != nil {
		return e.ProvisionHelmChart.Provision(kube, namespace, id)
	} else {
		glog.Errorln("Missing provision type")
		return nil, errors.New("Failed to provision")
	}
}

func (p ProvisionExistingClusterService) Provision(kube Kube, namespace string, id string) (*Instance, error) {
	URL := fmt.Sprintf("%s.%s.svc.cluster.local", p.Name, p.Namespace)

	instance := Instance{id, nil, &CoordinatesClusterURL{URL: URL}, &ResourcesNoResource{}, nil}
	return &instance, nil
}

func (p ProvisionNonClusterURL) Provision(kube Kube, namespace string, id string) (*Instance, error) {
	URL := p.URL
	instance := Instance{id, &CoordinatesExternalURL{URL: URL}, nil, &ResourcesNoResource{}, nil}
	return &instance, nil
}

func (p ProvisionHelmChart) Provision(kube Kube, namespace string, id string) (*Instance, error) {

	URL := p.URL
	destdir := kube.(*RealKube).Tmpdir
	tarroot, err := chartdl.DownloadChart(destdir, URL)
	if err == nil {
		glog.Infof("Chart at URL <%s> downloaded to <%s>", URL, tarroot)
	} else {
		glog.Infof("Chart at URL <%s> download failed <%s>", URL, err)
	}
	instance := Instance{id, &CoordinatesExternalURL{URL: "http://dummy.default.svc.cluster.local"}, nil, &ResourcesNoResource{}, nil}
	return &instance, nil
}

// TODO rename to InOrder
type ByOrder []v1.ConfigMap

func (a ByOrder) Len() int      { return len(a) }
func (a ByOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool {
	return a[i].ObjectMeta.Labels["mesitis/order"] < a[j].ObjectMeta.Labels["mesitis/order"]
}

func (p ProvisionNewClusterObjects) Provision(kube Kube, namespace string, id string) (*Instance, error) {

	const dataKey = "wrapped-resource"

	obj := p

	glog.Infof("Attempting to find config maps matching labelselector: %s\n", obj.LabelSelector)
	list, err := kube.ListConfigMaps(namespace, obj.LabelSelector)
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
		case "wrapped-pod":
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
		case "wrapped-deployment":
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

		case "wrapped-service":
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

		case "wrapped-configmap":
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

		case "wrapped-secret":
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
	instance := Instance{id, nil, &CoordinatesClusterURL{URL: URL}, nil, &pcfo}

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
