// Package admission ...
package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/m198799/timezone-webhook/internal"
	"github.com/m198799/timezone-webhook/internal/inject"
	"github.com/m198799/timezone-webhook/internal/log"
)

const currentVersion = "202312261204"

var (
	filterNsMap = make(map[string]bool)
)

// RequestsHandler ...
type RequestsHandler struct {
	DefaultTimezone           string
	DefaultInjectionStrategy  inject.InjectionStrategy
	InjectByDefault           bool
	HostPathPrefix            string
	LocalTimePath             string
	ConfigMapName             string
	ZoneInfoNamespaces        string
	InjectNamespaceAnnotation bool
	clientSet                 kubernetes.Interface
}

// Server ..
type Server struct {
	TLSCertFile string
	TLSKeyFile  string
	Address     string
	Handler     RequestsHandler
	Verbose     bool
}

// NewAdmissionServer ...
func NewAdmissionServer() *Server {
	return &Server{
		TLSCertFile: "/run/secrets/tls/tls.crt",
		TLSKeyFile:  "/run/secrets/tls/tls.key",
		Address:     ":8443",
		Handler:     NewRequestsHandler(),
		Verbose:     false,
	}
}

// NewRequestsHandler ...
func NewRequestsHandler() RequestsHandler {
	return RequestsHandler{
		DefaultTimezone:          internal.DefaultTimezone,
		DefaultInjectionStrategy: inject.DefaultInjectionStrategy,
		InjectByDefault:          true,
		HostPathPrefix:           inject.DefaultHostPathPrefix,
		LocalTimePath:            inject.DefaultLocalTimePath,
		ConfigMapName:            inject.DefaultZoneInfoConfigmapName,
	}
}

// getKubeConfig read config file
func getKubeConfig(kubeConfPath string) (*restclient.Config, error) {
	if kubeConfPath == "" {
		log.Info("--kubeconfig not specified. Using the inClusterConfig. This might not work.")
		kubeconfig, err := restclient.InClusterConfig()
		if err == nil {
			return kubeconfig, nil
		}
		log.Warn("error creating inClusterConfig, falling back to default config.")
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
}

// InitializeClientSet init client set for kubeconfig path
func (h *RequestsHandler) InitializeClientSet(kubeConfPath string) error {
	config, err := getKubeConfig(kubeConfPath)
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %v", err)
	}

	h.clientSet = clientset
	return nil
}

// initWebHookNamespace init webhook namespace
func (h *RequestsHandler) initWebHookNamespace() {
	if h.ZoneInfoNamespaces == "" {
		h.ZoneInfoNamespaces = inject.DefaultNamespace
	}
	log.Info("webhook work in namespaces: ", h.ZoneInfoNamespaces)
	if h.ZoneInfoNamespaces != "" {
		filterNamespaces := strings.Split(h.ZoneInfoNamespaces, ",")
		for _, ns := range filterNamespaces {
			filterNsMap[ns] = true
		}
	}
}

// GetClientSet ...
func (h *RequestsHandler) GetClientSet() kubernetes.Interface {
	return h.clientSet
}

// Start listen address to receive api-server webhook
func (h *Server) Start(kubeconfigFlag string) error {
	if err := h.Handler.InitializeClientSet(kubeconfigFlag); err != nil {
		return fmt.Errorf("failed to setup connection with kubernetes api: %w", err)
	}
	h.Handler.initWebHookNamespace()
	if err := inject.InitZoneInfoConfigmap(context.TODO(), h.Handler.GetClientSet(), h.Handler.ConfigMapName, strings.Split(h.Handler.ZoneInfoNamespaces, ",")); err != nil {
		return fmt.Errorf("failed to init zoneinfo to configmap: %w", err)
	}
	log.Info("Listening on ", "address:", h.Address)

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.Handler.handleFunc)
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/version", h.version)

	server := &http.Server{
		Addr:    h.Address,
		Handler: mux,
	}
	return server.ListenAndServeTLS(h.TLSCertFile, h.TLSKeyFile)
}

// health is api-server check webhook server is alive
func (h *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// version is api-server check webhook version
func (h *Server) version(w http.ResponseWriter, _ *http.Request) {
	_, err := w.Write([]byte(currentVersion))
	if err != nil {
		log.Error("failed to write response to output http stream", "err", err)
		http.Error(w, fmt.Sprintf("failed to write response: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleFunc is handler webhook request
func (h *RequestsHandler) handleFunc(w http.ResponseWriter, r *http.Request) {
	log.Info(fmt.Sprintf("start handler webhook request for method %s", r.Method))
	review, header, err := h.readAdmissionReview(r)
	if err != nil {
		log.Warn("failed to parse review:", zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to parse admission review from request, error: %s", err.Error()), header)
		return
	}
	reviewResponse := admission.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: &admission.AdmissionResponse{
			UID: review.Request.UID,
		},
	}

	reviewResponse.Response.Allowed = true

	if patches, err := h.handleAdmissionReview(r.Context(), review); err != nil {
		log.Warn("rejecting request:", "Namespace: ", review.Request.Namespace, "Name: ", review.Request.Name, "err: ", err)
		reviewResponse.Response.Allowed = false
		reviewResponse.Response.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else if patches != nil {
		patchBytes, err := json.Marshal(patches)
		if err != nil {
			log.Error("failed to marshal json patch", zap.Any("patches", patches), zap.Error(err))
			http.Error(w, fmt.Sprintf("could not marshal JSON patch: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		reviewResponse.Response.Patch = patchBytes
		reviewResponse.Response.PatchType = new(admission.PatchType)
		*reviewResponse.Response.PatchType = admission.PatchTypeJSONPatch
		log.Info("accepting request patches generated", " Namespace: ", review.Request.Namespace)
	}

	bytes, err := json.Marshal(&reviewResponse)
	if err != nil {
		log.Error("failed to marshal response review", zap.Any("reviewResponse", reviewResponse), zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to marshal response review: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(bytes)
	if err != nil {
		log.Error("failed to write response to output http stream", zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to write response: %s", err.Error()), http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
}

// IsFilterNamespace is filter this namespace, true is filter,false is not filter
func (h *RequestsHandler) IsFilterNamespace(namespace string) bool {
	if _, ok := filterNsMap[namespace]; ok {
		return false
	}
	return true
}
