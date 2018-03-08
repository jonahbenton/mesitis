package controller

import (
	"errors"
	"reflect"
	"sync"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/brokerapi"
)

type Controller interface {
	Catalog() (*brokerapi.Catalog, error)

	GetServiceInstanceLastOperation(instanceID, serviceID, planID, operation string) (*brokerapi.LastOperationResponse, error)
	CreateServiceInstance(instanceID string, req *brokerapi.CreateServiceInstanceRequest) (*brokerapi.CreateServiceInstanceResponse, error)
	RemoveServiceInstance(instanceID, serviceID, planID string, acceptsIncomplete bool) (*brokerapi.DeleteServiceInstanceResponse, error)

	Bind(instanceID, bindingID string, req *brokerapi.BindingRequest) (*brokerapi.CreateServiceBindingResponse, error)
	UnBind(instanceID, bindingID, serviceID, planID string) error
}

type ProductionController struct {
	Namespace string
	rwMutex   sync.RWMutex
	Storage   Storage
	Kube      Kube
	Tmpdir    string
}

type ControllerOptions struct {
	StorageType          string
	StorageRedisAddress  string
	StorageRedisPassword string
	StorageRedisDatabase string
	Name                 string
	Namespace            string
}

func CreateProductionController(name, namespace string, storage Storage, tmpdir string) Controller {

	return &ProductionController{
		Namespace: namespace,
		Kube:      &RealKube{},
		Storage:   storage,
		Tmpdir:    tmpdir,
	}
}

func (c *ProductionController) Catalog() (*brokerapi.Catalog, error) {

	// Catalog() may be called multiple times, concurrently and consecutively
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	var catalog *[]Entry
	var err error

	if catalog, err = LoadCatalogFromConfigMaps(&RealKube{}, c.Namespace); err != nil {
		glog.Errorf("Failed to load catalog: %s", err)
		return &brokerapi.Catalog{}, nil
	}

	glog.Infof("Catalog loaded: %s", catalog)
	// TODO logging each entry should be debug
	// TODO is it unnecessary to point to the entry
	var entry *Entry
	for i := len(*catalog) - 1; i >= 0; i-- {
		entry = &(*catalog)[i]
		glog.Infof("Entry: %s", entry.String())
	}

	services := make([]*brokerapi.Service, 0)
	for _, s := range *catalog {
		service := &brokerapi.Service{
			Name:        s.serviceName(),
			ID:          s.UUID,
			Description: s.Description,
			Plans: []brokerapi.ServicePlan{{
				Name:        s.planName(),
				ID:          s.UUID,
				Description: s.planDescription(),
				Free:        true,
			},
			},
			Bindable:       true,
			PlanUpdateable: true,
		}
		services = append(services, service)
	}
	bc := &brokerapi.Catalog{Services: services}
	return bc, nil
}

/*
CreateServiceInstance gets called with:

id: Catalog assigned ID for this provisioned instance, ex f1d0c814-9d40-4a60-ae0a-ebaadd9089ae
req: struct with the following elements
 	OrgID             - random uuid?, ex 9620207a-00f0-11e8-b216-5254008bf056
	PlanID            - plan selected by the caller, ex 3
    ServiceID         - service selected by the caller, ex 3
	SpaceID           - random uuid, ex 9620207a-00f0-11e8-b216-5254008bf056
	Parameters        - name value pairs provided by the request, ex map[param-1:value-1 param-2:value-2]
	AcceptsIncomplete - unclear, usually false
	ContextProfile    - struct with platform and namespace fields: "kubernetes" and the namespace the instance is being created in
we can return an empty object, nil as response on success, or nil and an error object on error

*/
func (c *ProductionController) CreateServiceInstance(id string, req *brokerapi.CreateServiceInstanceRequest) (*brokerapi.CreateServiceInstanceResponse, error) {
	// CreateServiceInstance() may be called concurrently
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if InstanceExists(c.Storage, id) {
		glog.Infof("Instance %s already exists, returning\n", id)
		return &brokerapi.CreateServiceInstanceResponse{}, nil
	}

	var catalog *[]Entry
	var err error

	if catalog, err = LoadCatalogFromConfigMaps(&RealKube{}, c.Namespace); err != nil {
		glog.Errorf("Failed to load catalog: %s", err)
		return nil, err
	}

	// TODO make debug
	// TODO use range?
	glog.Infof("Catalog loaded: %s", catalog)
	var entry *Entry
	for i := len(*catalog) - 1; i >= 0; i-- {
		entry = &(*catalog)[i]
		glog.Infof("Entry: %s", entry.String())
	}

	// TODO use range?
	// check on the plan and service. do those exist? if not exist, return error
	for i := len(*catalog) - 1; i >= 0; i-- {
		entry = &(*catalog)[i]
		if entry.UUID == req.PlanID {
			break
		}
	}
	if entry.UUID != req.PlanID {
		glog.Errorf("CreateServiceInstance %s for plan %s rejected, no matching plan.", id, req.PlanID)
		return nil, errors.New("No matching plan.")
	}

	// TODO debug
	glog.Infof("Found matching catalog entry: %s", entry.String())
	glog.Infof("Entry.ProvisionObj: %s", reflect.TypeOf(entry.ProvisionObj).String())
	glog.Infof("Entry.CredentialObj: %s", reflect.TypeOf(entry.CredentialObj).String())

	// does the calling namespace exist in the whitelist for the given service and plan.
	// if no, return error
	callerNamespace := req.ContextProfile.Namespace
	allowed := false
	for _, n := range entry.Whitelist {
		if n == callerNamespace {
			allowed = true
		}
	}
	// TODO make the same error message
	if !allowed {
		glog.Errorf("CreateServiceInstance %s for plan %s rejected, namespace %s not in whitelist %s", id, req.PlanID, req.ContextProfile.Namespace, entry.Whitelist)
		return nil, errors.New("Namespace not in whitelist.")
	}
	glog.Infof("Provisioning Service Instance from: %s", entry.String())

	var instance *Instance

	switch entry.ProvisionObj.(type) {
	case ProvisionNonClusterURL:
		instance, err = entry.ProvisionObj.(ProvisionNonClusterURL).Provision(entry, id, c.Kube, c.Namespace)
	case ProvisionExistingClusterService:
		instance, err = entry.ProvisionObj.(ProvisionExistingClusterService).Provision(entry, id, c.Kube, c.Namespace)
	case ProvisionNewClusterObjects:
		instance, err = entry.ProvisionObj.(ProvisionNewClusterObjects).Provision(entry, id, c.Kube, c.Namespace)
	default:
		// TODO make same error message
		glog.Errorln("Unknown provision type")
		return nil, errors.New("Failed to provision")
	}
	if err != nil {
		glog.Errorf("Provisioning failed %s: %s", id, err)
		return nil, err
	}

	// TODO better to save the instance first, then update after provisioning
	err = SaveInstance(c.Storage, id, instance)
	if err != nil {
		glog.Errorf("Failed to save instance %s: %s", id, err)
		return nil, err
	}

	return &brokerapi.CreateServiceInstanceResponse{}, nil
}

// unclear under what circumstances this gets called
func (c *ProductionController) GetServiceInstanceLastOperation(instanceID, serviceID, planID, operation string) (*brokerapi.LastOperationResponse, error) {
	return nil, errors.New("Unimplemented")
}

func (c *ProductionController) RemoveServiceInstance(instanceID, serviceID, planID string, acceptsIncomplete bool) (*brokerapi.DeleteServiceInstanceResponse, error) {
	// RemoveServiceInstance() may be called concurrently
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	// if the instance exists, delete any provisioned resources
	pr, err := LoadInstance(c.Storage, instanceID)
	if err == nil {
		switch pr.ResourcesObj.(type) {
		case ResourcesKubeObjectList:
			glog.Infof("Resources associated with this ID are: %s", pr.String())
			// reverse order
			// TODO update and save after each deprovision? probably not necessary
			for i := len(pr.ResourcesObj.(ResourcesKubeObjectList)) - 1; i >= 0; i-- {
				po := pr.ResourcesObj.(ResourcesKubeObjectList)[i]
				switch po.Kind {
				case "wrapped-pod":
					err = c.Kube.DeletePod(po.Namespace, po.Name)
				case "wrapped-deployment":
					err = c.Kube.DeleteDeployment(po.Namespace, po.Name)
				case "wrapped-service":
					err = c.Kube.DeleteService(po.Namespace, po.Name)
				case "wrapped-configmap":
					err = c.Kube.DeleteConfigMap(po.Namespace, po.Name)
				case "wrapped-secret":
					err = c.Kube.DeleteSecret(po.Namespace, po.Name)
				default:
					glog.Errorf("Unable to delete provisioned object kind: %s", po.Kind)
				}
				if err != nil {
					glog.Errorf("Failed to delete provisioned object: %s", err)
				}
			}
		case ResourcesNoResource:
			// nothing to do
		default:
			// nothing to do
		}
	} else {
		glog.Errorf("Unable to find provisioned objects!")
	}

	err = DeleteInstance(c.Storage, instanceID)
	if err != nil {
		glog.Errorf("Failed to delete instance %s in storage: %s", instanceID, err)
	}

	return &brokerapi.DeleteServiceInstanceResponse{}, nil
}

/*
Bind gets called with the instanceID, bindingID and

type BindingRequest struct {
	AppGUID      string                 `json:"app_guid,omitempty"`
	PlanID       string                 `json:"plan_id,omitempty"`
	ServiceID    string                 `json:"service_id,omitempty"`
	BindResource map[string]interface{} `json:"bind_resource,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`

Bind:
	instanceID: 30ab7369-3e96-426a-bc97-6ee5bc00d6ad
	bindingID: ccce6f06-221b-4e44-a185-68ca3218350c
	&{ PlanID: 2 ServiceID: 1 map[app_guid:b8797dff-04c1-11e8-9a24-0800273be027] map[instanceId:30ab7369-3e96-426a-bc97-6ee5bc00d6ad]}
}
*/
func (c *ProductionController) Bind(instanceID, bindingID string, req *brokerapi.BindingRequest) (*brokerapi.CreateServiceBindingResponse, error) {
	// Bind() may be called concurrently
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	// if bindingID exists, return prior binding data
	// TODO do a BindingExists
	if BindingExists(c.Storage, bindingID) {
		if b, err := LoadBinding(c.Storage, bindingID); err == nil {
			glog.Infof("Binding %s exists, returning prior bind data.", bindingID)
			return &brokerapi.CreateServiceBindingResponse{Credentials: b.Credential}, nil
		}
	} else {
		glog.Infof("Binding %s not found, creating.", bindingID)
	}

	instance, err := LoadInstance(c.Storage, instanceID)
	if err != nil {
		glog.Errorf("No instance %s to bind %s: %s", instanceID, bindingID, err)
		return nil, err
	}

	// TODO debug
	glog.Infof("Retrieved instance to bind:", instance.String())
	glog.Infof("Retrieved entry from instance:", instance.Entry.String())

	var cred brokerapi.Credential

	// TODO coordinates needs to be included in the Credential
	// TODO needs to support port and protocol, etc
	var URL string

	switch instance.CoordinatesObj.(type) {
	case CoordinatesClusterURL:
		URL = instance.CoordinatesObj.(CoordinatesClusterURL).URL
	case CoordinatesExternalURL:
		URL = instance.CoordinatesObj.(CoordinatesExternalURL).URL
	default:
		glog.Errorf("Unrecognized CoordinatesObj type: %s", reflect.TypeOf(instance.CoordinatesObj).String())
	}

	// TODO serialization mechanism to not write passwords to logs

	switch instance.Entry.CredentialObj.(type) {
	case CredentialFromCatalog:
		cred = brokerapi.Credential{
			"URL":      URL,
			"Username": instance.Entry.CredentialObj.(CredentialFromCatalog).Username,
			"Password": instance.Entry.CredentialObj.(CredentialFromCatalog).Password,
		}
	case CredentialFromClusterSecret:
		name := instance.Entry.CredentialObj.(CredentialFromClusterSecret).SecretName
		secret, err := c.Kube.GetSecret(c.Namespace, name)
		if err == nil {
			cred = brokerapi.Credential{}
			for key, value := range secret.Data {
				cred[key] = value
			}
		} else {
			glog.Errorf("Unable to find secret %s for CredentialFromClusterSecret for binding %s", name, bindingID)
			return nil, err
		}
	case CredentialNoCredential:
		cred = brokerapi.Credential{
			"URL": URL,
		}
	default:
		return nil, errors.New("Unknown credential type.")
	}

	glog.Infof("Creating Binding: %s", bindingID)
	binding := &Binding{instance, bindingID, cred}
	if err := SaveBinding(c.Storage, bindingID, binding); err != nil {
		glog.Errorf("Failed to save Binding %s: %s", bindingID, err)
	}

	return &brokerapi.CreateServiceBindingResponse{Credentials: cred}, nil
}

func (c *ProductionController) UnBind(instanceID, bindingID, serviceID, planID string) error {
	// Unbind() may be called concurrently
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if _, err := LoadBinding(c.Storage, bindingID); err == nil {
		glog.Infof("Binding %s exists, attempt to delete.", bindingID)
		if err := DeleteBinding(c.Storage, bindingID); err == nil {
			glog.Infof("Binding %s deleted.", bindingID)
		} else {
			glog.Errorf("Error deleting Binding %s: %s", bindingID, err)
		}
	} else {
		glog.Infof("Binding %s not found, assume already deleted.", bindingID)
	}

	return nil
}
