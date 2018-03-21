package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/golang/glog"
	v1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jonahbenton/mesitis/pkg/chartdl"
)

// TODO is it necessary to have these distinct create functions?
// TODO can't it be just CreateObjectFromJSON?
type Kube interface {
	ListConfigMaps(namespace, labelSelector string) (*v1.ConfigMapList, error)
	CreatePodFromJSON(string, string) (*v1.Pod, error)
	CreateServiceFromJSON(string, string) (*v1.Service, error)
	CreateDeploymentFromJSON(string, string) (*v1beta1.Deployment, error)
	CreateConfigMapFromJSON(string, string) (*v1.ConfigMap, error)
	CreateSecretFromJSON(string, string) (*v1.Secret, error)
	DeletePod(namespace, name string) error
	DeleteService(namespace, name string) error
	DeleteDeployment(namespace, name string) error
	DeleteConfigMap(namespace, name string) error
	DeleteSecret(namespace, name string) error
	PodExists(string, string) bool
	ServiceExists(string, string) bool
	DeploymentExists(string, string) bool
	ConfigMapExists(string, string) bool
	SecretExists(string, string) bool
	GetSecret(namespace, name string) (*v1.Secret, error)
}

type RealKube struct {
	Tmpdir string
}

type Provision interface {
	Provision(entry *Entry, id string, kube Kube, namespace string) (*Instance, error)
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

// TODO don't panic
func kubeapi() *kubernetes.Clientset {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

// TODO is this necessary?
func kubeError(err error) {
	if k8serr.IsNotFound(err) {
		glog.Errorf("Object not found: %s", err)
	} else if statusError, isStatus := err.(*k8serr.StatusError); isStatus {
		glog.Errorf("Error creating object %s\n", statusError.ErrStatus.Message)
	} else if err != nil {
		glog.Errorf(err.Error())
	}
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func (k *RealKube) GetSecret(namespace, name string) (*v1.Secret, error) {

	secret, err := kubeapi().CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Failed to load secret: %s", err)
		return nil, err
	}
	return secret, nil
}

func (k *RealKube) ListConfigMaps(namespace, labelSelector string) (*v1.ConfigMapList, error) {

	list, err := kubeapi().CoreV1().ConfigMaps(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		// TODO use in catalog is not guaranteed
		glog.Errorf("Failed to find config maps from which to load the catalog: %s", err)
		return nil, err
	}
	return list, nil
}

func (k *RealKube) CreatePodFromJSON(namespace string, JSON string) (*v1.Pod, error) {

	var p v1.Pod
	if err := json.Unmarshal([]byte(JSON), &p); err != nil {
		glog.Errorf("Failed to unmarshal a valid pod from JSON: %s\n%s\n", err, JSON)
		return nil, err
	}

	if pod, err := kubeapi().CoreV1().Pods(namespace).Create(&p); err == nil {
		return pod, nil
	} else {
		glog.Errorf("Failed to create pod: %s", err)
		return nil, err
	}
}

func (k *RealKube) CreateServiceFromJSON(namespace string, JSON string) (*v1.Service, error) {

	var s v1.Service
	if err := json.Unmarshal([]byte(JSON), &s); err != nil {
		glog.Errorf("Failed to unmarshal a valid service from configmap: %s\n%s\n", err, JSON)
		return nil, err
	}

	if service, err := kubeapi().CoreV1().Services(namespace).Create(&s); err == nil {
		return service, nil
	} else {
		glog.Errorf("Failed to create service: %s", err)
		return nil, err
	}
}

func (k *RealKube) CreateDeploymentFromJSON(namespace string, JSON string) (*v1beta1.Deployment, error) {

	var d v1beta1.Deployment
	if err := json.Unmarshal([]byte(JSON), &d); err != nil {
		glog.Errorf("Failed to unmarshal a valid deployment from ConfigMap: %s\n%s\n", err, JSON)
		return nil, err
	}

	if deployment, err := kubeapi().AppsV1beta1().Deployments(namespace).Create(&d); err == nil {
		return deployment, nil
	} else {
		glog.Errorf("Failed to create deployment: %s", err)
		return nil, err
	}
}

func (k *RealKube) CreateConfigMapFromJSON(namespace string, JSON string) (*v1.ConfigMap, error) {

	var c v1.ConfigMap
	if err := json.Unmarshal([]byte(JSON), &c); err != nil {
		glog.Errorf("Failed to unmarshal a valid ConfigMap from configmap: %s\n%s\n", err, JSON)
		return nil, err
	}

	if configMap, err := kubeapi().CoreV1().ConfigMaps(namespace).Create(&c); err == nil {
		return configMap, nil
	} else {
		glog.Errorf("Failed to create ConfigMap: %s", err)
		return nil, err
	}
}

func (k *RealKube) CreateSecretFromJSON(namespace string, JSON string) (*v1.Secret, error) {

	var s v1.Secret
	if err := json.Unmarshal([]byte(JSON), &s); err != nil {
		glog.Errorf("Failed to unmarshal a valid secret from configmap: %s\n%s\n", err, JSON)
		return nil, err
	}

	if secret, err := kubeapi().CoreV1().Secrets(namespace).Create(&s); err == nil {
		return secret, nil
	} else {
		glog.Errorf("Failed to create secret: %s", err)
		return nil, err
	}
}

func (k *RealKube) DeletePod(namespace, name string) error {

	fg := metav1.DeletePropagationForeground
	err := kubeapi().CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{
		GracePeriodSeconds: &[]int64{0}[0],
		PropagationPolicy:  &fg})

	if err != nil {
		glog.Errorf("Failed to delete provisioned pod: %s", err)
	}

	return err
}

func (k *RealKube) DeleteDeployment(namespace, name string) error {

	fg := metav1.DeletePropagationForeground
	err := kubeapi().AppsV1beta1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{
		GracePeriodSeconds: &[]int64{0}[0],
		PropagationPolicy:  &fg})

	if err != nil {
		glog.Errorf("Failed to delete provisioned deployment: %s", err)
	}

	return err
}

func (k *RealKube) DeleteService(namespace, name string) error {

	fg := metav1.DeletePropagationForeground
	err := kubeapi().CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{
		GracePeriodSeconds: &[]int64{0}[0],
		PropagationPolicy:  &fg})

	if err != nil {
		glog.Errorf("Failed to delete provisioned service: %s", err)
	}

	return err
}

func (k *RealKube) DeleteConfigMap(namespace, name string) error {

	fg := metav1.DeletePropagationForeground
	err := kubeapi().CoreV1().ConfigMaps(namespace).Delete(name, &metav1.DeleteOptions{
		GracePeriodSeconds: &[]int64{0}[0],
		PropagationPolicy:  &fg})

	if err != nil {
		glog.Errorf("Failed to delete provisioned ConfigMap: %s", err)
	}

	return err
}

func (k *RealKube) DeleteSecret(namespace, name string) error {

	fg := metav1.DeletePropagationForeground
	err := kubeapi().CoreV1().Secrets(namespace).Delete(name, &metav1.DeleteOptions{
		GracePeriodSeconds: &[]int64{0}[0],
		PropagationPolicy:  &fg})

	if err != nil {
		glog.Errorf("Failed to delete provisioned secret: %s", err)
	}

	return err
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func (k *RealKube) PodExists(namespace, name string) bool {

	pod, err := kubeapi().CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if pod != nil && err == nil {
		return true
	}
	return false
}

func (k *RealKube) DeploymentExists(namespace, name string) bool {

	deployment, err := kubeapi().AppsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if deployment != nil && err == nil {
		return true
	}
	return false
}

func (k *RealKube) ServiceExists(namespace, name string) bool {

	service, err := kubeapi().CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if service != nil && err == nil {
		return true
	}
	return false
}

func (k *RealKube) ConfigMapExists(namespace, name string) bool {

	configMap, err := kubeapi().CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if configMap != nil && err == nil {
		return true
	}
	return false
}

func (k *RealKube) SecretExists(namespace, name string) bool {

	secret, err := kubeapi().CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if secret != nil && err == nil {
		return true
	}
	return false
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func LoadCatalogFromConfigMaps(k Kube, namespace string) (*[]Entry, error) {
	var catalog []Entry
	var err error

	const labelSelector = "mesitis/kind=catalog-entry"
	const dataKey = "wrapped-resource"

	list, err := k.ListConfigMaps(namespace, labelSelector)
	if err != nil {
		glog.Errorf("Failed to find config maps from which to load the catalog: %s", err)
		return nil, err
	}

	glog.Infof("Found <%d> entries.", len(list.Items))
	for _, cm := range list.Items {
		js := cm.Data[dataKey]

		var entry *Entry
		if err := json.Unmarshal([]byte(js), &entry); err != nil {
			glog.Errorf("Failed to unmarshal catalog data <%s> from ConfigMap: %s", js, err)
			continue
		}
		catalog = append(catalog, *entry)
	}

	return &catalog, nil
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
			case "wrapped-pod":
				err = kube.DeletePod(po.Namespace, po.Name)
			case "wrapped-deployment":
				err = kube.DeleteDeployment(po.Namespace, po.Name)
			case "wrapped-service":
				err = kube.DeleteService(po.Namespace, po.Name)
			case "wrapped-configmap":
				err = kube.DeleteConfigMap(po.Namespace, po.Name)
			case "wrapped-secret":
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
		return e.ProvisionExistingClusterService.Provision(e, id, kube, namespace)
	} else if e.ProvisionNonClusterURL != nil {
		return e.ProvisionNonClusterURL.Provision(e, id, kube, namespace)
	} else if e.ProvisionNewClusterObjects != nil {
		return e.ProvisionNewClusterObjects.Provision(e, id, kube, namespace)
	} else if e.ProvisionHelmChart != nil {
		return e.ProvisionHelmChart.Provision(e, id, kube, namespace)
	} else {
		glog.Errorln("Missing provision type")
		return nil, errors.New("Failed to provision")
	}
}

func (p ProvisionExistingClusterService) Provision(entry *Entry, id string, kube Kube, namespace string) (*Instance, error) {
	URL := fmt.Sprintf("%s.%s.svc.cluster.local", p.Name, p.Namespace)

	instance := Instance{*entry, id, nil, &CoordinatesClusterURL{URL: URL}, &ResourcesNoResource{}, nil}
	return &instance, nil
}

func (p ProvisionNonClusterURL) Provision(entry *Entry, id string, kube Kube, namespace string) (*Instance, error) {
	URL := p.URL
	instance := Instance{*entry, id, &CoordinatesExternalURL{URL: URL}, nil, &ResourcesNoResource{}, nil}
	return &instance, nil
}

func (p ProvisionHelmChart) Provision(entry *Entry, id string, kube Kube, namespace string) (*Instance, error) {

	URL := p.URL
	destdir := kube.(*RealKube).Tmpdir
	tarroot, err := chartdl.DownloadChart(destdir, URL)
	if err == nil {
		glog.Infof("Chart at URL <%s> downloaded to <%s>", URL, tarroot)
	} else {
		glog.Infof("Chart at URL <%s> download failed <%s>", URL, err)
	}
	instance := Instance{*entry, id, &CoordinatesExternalURL{URL: "http://dummy.default.svc.cluster.local"}, nil, &ResourcesNoResource{}, nil}
	return &instance, nil
}

// TODO rename to InOrder
type ByOrder []v1.ConfigMap

func (a ByOrder) Len() int      { return len(a) }
func (a ByOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool {
	return a[i].ObjectMeta.Labels["mesitis/order"] < a[j].ObjectMeta.Labels["mesitis/order"]
}

func (p ProvisionNewClusterObjects) Provision(entry *Entry, id string, kube Kube, namespace string) (*Instance, error) {

	const dataKey = "wrapped-resource"

	obj := entry.ProvisionNewClusterObjects

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
						Kind:      "wrapped-pod",
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
						Kind:      "wrapped-deployment",
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
						Kind:      "wrapped-service",
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
						Kind:      "wrapped-configmap",
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
						Kind:      "wrapped-secret",
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
	instance := Instance{*entry, id, nil, &CoordinatesClusterURL{URL: URL}, nil, &pcfo}

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
