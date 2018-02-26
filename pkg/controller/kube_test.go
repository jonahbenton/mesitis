package controller

//type Kube interface {
//	ListConfigMaps(namespace, labelSelector string) (*v1.ConfigMapList, error)
//	CreatePodFromJSON(string, string) (*v1.Pod, error)
//	CreateServiceFromJSON(string, string) (*v1.Service, error)
//	CreateDeploymentFromJSON(string, string) (*v1beta1.Deployment, error)
//	DeletePod(namespace, name string) error
//	DeleteService(namespace, name string) error
//	DeleteDeployment(namespace, name string) error
//	GetSecret(namespace, name string) (*v1.Secret, error)
//}

type FakeKube struct{}
