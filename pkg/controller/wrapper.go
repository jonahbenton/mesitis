package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/brokerapi"
)

type emptyJSON struct{}

type ControllerHTTPWrapper struct {
	controller Controller
}

// TODO this logging needs to be V level trace
// TODO use this to eliminate the trace log in each dispatcher call
func headerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glog.Infof("U: %q\n", r.RequestURI)
		for k, v := range r.Header {
			glog.Infof("H: %q: %q\n", k, v)
		}
		next.ServeHTTP(w, r)
	})
}

func CreateHTTPWrapper(c Controller) http.Handler {

	var router = mux.NewRouter()

	cw := ControllerHTTPWrapper{
		controller: c,
	}

	router.HandleFunc("/v2/catalog", cw.catalog).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}/last_operation", cw.getServiceInstanceLastOperation).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}", cw.createServiceInstance).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}", cw.removeServiceInstance).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", cw.bind).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", cw.unBind).Methods("DELETE")

	// TODO why is this a func reference, not a function call?
	router.Use(headerMiddleware)

	return router
}

func (cw *ControllerHTTPWrapper) catalog(w http.ResponseWriter, r *http.Request) {

	if result, err := cw.controller.Catalog(); err == nil {
		sendJSONObject(w, http.StatusOK, result)
	} else {
		sendJSONObject(w, http.StatusBadRequest, err)
	}
}

func (cw *ControllerHTTPWrapper) getServiceInstanceLastOperation(w http.ResponseWriter, r *http.Request) {

	instanceID := mux.Vars(r)["instance_id"]
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	planID := q.Get("plan_id")
	operation := q.Get("operation")

	if result, err := cw.controller.GetServiceInstanceLastOperation(instanceID, serviceID, planID, operation); err == nil {
		sendJSONObject(w, http.StatusOK, result)
	} else {
		sendJSONObject(w, http.StatusBadRequest, err)
	}
}

func (cw *ControllerHTTPWrapper) createServiceInstance(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["instance_id"]

	var req brokerapi.CreateServiceInstanceRequest
	if err := getJSONObject(r, &req); err != nil {
		glog.Errorf("error unmarshalling: %v", err)
		sendJSONObject(w, http.StatusBadRequest, err)
		return
	}

	if result, err := cw.controller.CreateServiceInstance(id, &req); err == nil {
		sendJSONObject(w, http.StatusCreated, result)
	} else {
		sendJSONObject(w, http.StatusBadRequest, err)
	}
}

func (cw *ControllerHTTPWrapper) removeServiceInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := mux.Vars(r)["instance_id"]
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	planID := q.Get("plan_id")
	acceptsIncomplete := q.Get("accepts_incomplete") == "true"

	if result, err := cw.controller.RemoveServiceInstance(instanceID, serviceID, planID, acceptsIncomplete); err == nil {
		sendJSONObject(w, http.StatusOK, result)
	} else {
		sendJSONObject(w, http.StatusBadRequest, err)
	}
}

func (cw *ControllerHTTPWrapper) bind(w http.ResponseWriter, r *http.Request) {
	bindingID := mux.Vars(r)["binding_id"]
	instanceID := mux.Vars(r)["instance_id"]

	var req brokerapi.BindingRequest

	if err := getJSONObject(r, &req); err != nil {
		glog.Errorf("Failed to unmarshall request: %v", err)
		sendJSONObject(w, http.StatusBadRequest, err)
		return
	}

	if result, err := cw.controller.Bind(instanceID, bindingID, &req); err == nil {
		sendJSONObject(w, http.StatusOK, result)
	} else {
		sendJSONObject(w, http.StatusBadRequest, err)
	}
}

func (cw *ControllerHTTPWrapper) unBind(w http.ResponseWriter, r *http.Request) {
	instanceID := mux.Vars(r)["instance_id"]
	bindingID := mux.Vars(r)["binding_id"]
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	planID := q.Get("plan_id")

	if err := cw.controller.UnBind(instanceID, bindingID, serviceID, planID); err == nil {
		sendJSONObject(w, http.StatusOK, &emptyJSON{})
	} else {
		sendJSONObject(w, http.StatusBadRequest, err)
	}
}

func sendJSONObject(w http.ResponseWriter, code int, object interface{}) {
	data, err := json.Marshal(object)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, string(data))
}

func getJSONObject(r *http.Request, object interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, object)
	if err != nil {
		return err
	}

	return nil
}
