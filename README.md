## `mesitis`

[![Go Report Card](https://goreportcard.com/badge/github.com/jonahbenton/mesitis)](https://goreportcard.com/report/github.com/jonahbenton/mesitis)

### Introduction

Mesitis is a configuration-driven, team-oriented Service Broker for Kubernetes, built on the [Service Catalog](https://github.com/kubernetes-incubator/service-catalog) platform (currently in kubernetes-incubator).

Mesitis is also an experiment in seeing the Service Catalog platform as an *engagement protocol for teams within an org*, rather than an *integration protocol for services outside an org*. 

### Uses

In a multi-team microservice environment, teams act in roles of providers of and consumers for each others' services. 

With Mesitis, a providing team can 

* produce a catalog of services available to consuming teams, with configuration rather than code
* whitelist specific consuming teams for specific services
* specify provisioning of a service in or out of cluster on behalf of a consuming team with configuration rather than code
* ensure the automated, secure delivery of the coordinates and credentials needed by the consuming application
* track both provisioned services and bindings to understand users of its services
* ensure any resources and secrets are automatically deleted and deprovisioned when the consumer is finished consuming

Mesitis supports provisioners for:

* static out of cluster URLs
* static, shared in-cluster services, located by namespace and name
* individually provisionable JSON-based resource definitions 
* Helm charts (future)

Mesitis supports delivering credentials sourced from:

* individual catalog entries
* individual secrets in the provider namespace
* Vault (future)

### Motivation

The idea for Mesitis emerged in brainstorming with colleagues. In a multi-team microservice plant, it is difficult for teams to automate workflows around each other's work, in terms of service depender-dependee relationships- versioning and contracts- and in terms of the the lifecycle of secrets (credentials, etc) involved in providing/consuming those dependencies. 

### Demo

The walkthrough first has a providing team install Mesitis and some provider configuration as ConfigMaps into a namespace called "provider-ns". 

Then a consuming team, using a namespace called "client-ns", provisions and binds to a provider's service.

Then the consuming team unbinds from the provider's service, and the resources are released.

0. Mesitis integrates with the Service Catalog platform. 

    To install Service Catalog in a cluster using Helm:

    ```
    helm repo add svc-cat https://svc-catalog-charts.storage.googleapis.com
    helm install svc-cat/catalog     --name catalog --namespace catalog
    ```

1. Build Mesitis and push to your registry

    ```
    git clone https://github.com/jonahbenton/mesitis
	cd mesitis/cmd/mesitis
	go build .
	cd ../..
    docker build -t $REGISTRY/$REPO/mesitis:latest .
    docker push $REGISTRY/$REPO/mesitis:latest
    ```
	
2. Create the provider-ns and requisite service account, role, and role binding.

    ```
    kubectl create namespace provider-ns

    kubectl create -f resources/mesitis-user-role.yaml -n provider-ns
    kubectl create -f resources/mesitis-role-binding.yaml -n provider-ns
    ```
	
    Service Catalog has permissions to create and delete Secrets in consumer namespaces. Mesitis needs only permissions in the provider namespace(s), and does not need any special permissions in consumer namespaces.

3. Use the Mesitis chart to deploy the broker into the provider-ns namespace:

    ```
    helm install charts/mesitis --name mesitis --namespace provider-ns
    ```
	
    By default Mesitis uses an in-memory map to track state around service instances and bindings and any provisioned resources. Redis for storage is also available.

4. Build the demo service- a service that provides the current time- and push to your registry

5. Create the consuming team namespace

6. The consuming team views the catalog offered by the providing team

7. The consuming team provisions one of the services offered by the providing team

8. The consuming team binds to the provider service 

9. Use the consuming team's service, which depends on the provider's service

10. Unbind the consumer, and deprovision

11. Cleanup



### Configuration

Mesitis is configuration-driven. Assets like catalog entries and provisionable cluster resources are defined in individual ConfigMaps that live in the broker's namespace.

One type of ConfigMap is used to create Catalog entries. A catalog entry specifies a single service a providing team is making available to one or more consuming teams. 

	apiVersion: v1
	kind: ConfigMap
	metadata:
	  name: wrapped-entry-api-api-service
	  labels:
	    mesitis/kind: "catalog-entry"
	    mesitis/enabled: "true"
	data:
	  wrapped-resource: |
	    {
	        "team": "api",
	        "offering": "api-service",
	        "description":"An api service offered by the API team",
	        "uuid":"3",
	        "version":"1",
	        "whitelist": ["client-ns"],
	        "provisionkind": "ProvisionConfigMapObjects",
	        "provisiondata": {
	            "namespace": "api-ns",
	            "name": "api-service",
	            "labelSelector":"mesitis/offering=api-service"
	        },
	        "credentialkind": "CredentialFromCatalog",
	        "credentialdata": {
	            "username":"iamtheuser",
	            "password":"iamthepassword"
	        }
	    }


Provisionkind above refers to a ProvisionConfigMapResource. This is a Kubernetes resource, defined in JSON (YAML coming soon), encoded in the data area of the ConfigMap. Mesitis looks for these under the "wrapped-resource" key. 

	apiVersion: v1
	kind: ConfigMap
	metadata:
	  name: wrapped-api-service
	  labels:
	    mesitis/offering: "api-service"
	    mesitis/kind: "wrapped-service"
	    mesitis/enabled: "true"
	    mesitis/order: "2"
	data:
	  wrapped-resource: |
	    {
	      "apiVersion": "v1",
	      "kind": "Service",
	      "metadata": {
	        "name": "back-end-service",
	        "labels": {
	          "app": "back-end"
	        }
	      },
	      "spec": {
	        "selector":{
	            "app":"back-end"
	        },
	        "ports": [
	            {
	            "protocol": "TCP",
	            "port": 80,
	            "targetPort": 9000
	            }]
	        }
	    }

When provisioning the catalog entry above, the labelSelector allows Mesitis to locate all the ConfigMaps with a mesitis/offering of "api-service". Mesitis will filter them by
mesitis/enabled, sort them using mesitis/order, and then create them in the cluster.

Three kinds of catalog entries are currently supported:

* Out of cluster URL
* In cluster shared service, where provisioning is not controlled by Mesitis
* Dedicated in-cluster collection of Kube objects, where provisioning and deprovisioning is managed by Mesitis

Six kinds of basic Kubernetes objects are currently supported:

* Deployments
* Services
* Pods
* ReplicaSets
* ConfigMaps
* Secrets

Multiple instances of Mesitis can be installed in a cluster. Each should be owned by a team and run in its own namespace.


