package controller

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/brokerapi"
)

// An entry embedded in a kubernetes secret or configmap
type Entry struct {
	Team                            string                           `json:"team"`
	Offering                        string                           `json:"offering"`
	Description                     string                           `json:"description"`
	UUID                            string                           `json:"uuid"`
	Version                         string                           `json:"version"`
	Whitelist                       []string                         `json:"whitelist"`
	ProvisionExistingClusterService *ProvisionExistingClusterService `json:"ProvisionExistingClusterService"`
	ProvisionNonClusterURL          *ProvisionNonClusterURL          `json:"ProvisionNonClusterURL"`
	ProvisionNewClusterObjects      *ProvisionNewClusterObjects      `json:"ProvisionNewClusterObjects"`
	ProvisionHelmChart              *ProvisionHelmChart              `json:"ProvisionHelmChart"`
	CredentialFromClusterSecret     *CredentialFromClusterSecret     `json:"CredentialFromClusterSecret"`
	CredentialFromCatalog           *CredentialFromCatalog           `json:"CredentialFromCatalog"`
	CredentialFromVault             *CredentialFromVault             `json:"CredentialFromVault"`
	CredentialNoCredential          *CredentialNoCredential          `json:"CredentialNoCredential"`
}

type Instance struct {
	Entry
	InstanceID              string                   `json:"instanceID"`
	CoordinatesExternalURL  *CoordinatesExternalURL  `json:"CoordinatesExternalURL"`
	CoordinatesClusterURL   *CoordinatesClusterURL   `json:"CoordinatesClusterURL"`
	ResourcesNoResource     *ResourcesNoResource     `json:"ResourcesNoResource"`
	ResourcesKubeObjectList *ResourcesKubeObjectList `json:"ResourcesKubeObjectList"`
	ResourcesHelmRelease    *ResourcesHelmRelease    `json:"ResourcesHelmRelease"`
}

type Binding struct {
	*Instance
	BindingID  string `json:"bindingid"`
	Credential brokerapi.Credential
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

// An existing service living in the cluster
type ProvisionExistingClusterService struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// A URL that exists out of cluster
type ProvisionNonClusterURL struct {
	URL string `json:"url"`
}

// Provision objects wrapped in ConfigMaps in the broker namespace
// Namespace- into which should the objects be provisioned
// Namespace and Name: how to compose the URL for the provisioned service
// LabelSelector: how to pick out the specific ConfigMaps
type ProvisionNewClusterObjects struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	LabelSelector string `json:"labelselector"`
}

// Provision the chart specified in the struct below via the registry
// Chart from ChartURL retrieved and Installed in Namespace
// Assume there is a service called Name as a result of chart install
// otherwise would need to discover installed resources and infer coordinates
type ProvisionHelmChart struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	ChartURL  string `json:"charturl"`
}

// Credential lives in a Secret in the broker namespace
type CredentialFromClusterSecret struct {
	SecretName string `json:"secretname"`
}

// Credential is kept in line in the catalog
type CredentialFromCatalog struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Vault is the repository for this credential
type CredentialFromVault struct {
	VaultURL  string `json:"vaulturl"`
	VaultAuth string `json:"vaultauth"`
	VaultPath string `json:"vaultpath"`
}

// No credential is needed/used to reach this service
type CredentialNoCredential struct{}

type CoordinatesExternalURL struct {
	URL string `json:"url"`
}

type CoordinatesClusterURL struct {
	URL string `json:"url"`
}

type ResourcesNoResource struct{}

type ResourcesKubeObject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type ResourcesKubeObjectList []ResourcesKubeObject

type ResourcesHelmRelease struct {
	Namespace string `json:"Namespace"`
	Name      string `json:"Name"`
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func (c *CredentialFromClusterSecret) String() string {
	return fmt.Sprintf("{CredentialFromClusterSecret: %s}", c.SecretName)
}

func (c *CredentialFromCatalog) String() string {
	return fmt.Sprintf("{CredentialFromCatalog: %s}", c.Username)
}

func (c *CredentialFromVault) String() string {
	return fmt.Sprintf("{CredentialFromVault: %s:%s}", c.VaultURL, c.VaultPath)
}

func (c *CredentialNoCredential) String() string {
	return "{CredentialNoCredential}"
}

func (p *ProvisionExistingClusterService) String() string {
	return fmt.Sprintf("{ProvisionExistingClusterService: %s-%s}", p.Name, p.Namespace)
}

func (p *ProvisionNonClusterURL) String() string {
	return fmt.Sprintf("{ProvisionNonClusterURL: %s}", p.URL)
}

func (p *ProvisionNewClusterObjects) String() string {
	return fmt.Sprintf("{ProvisionNewClusterObjects: %s-%s}", p.Name, p.Namespace)
}

func (c *Entry) String() string {
	return fmt.Sprintf("{Entry: Team: %s, Offering: %s, Version: %s}",
		c.Team, c.Offering, c.Version)
}

func (p *Instance) String() string {
	return fmt.Sprintf("{Instance}")
}
