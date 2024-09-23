// Package admission ...
package admission

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	admission "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/m198799/timezone-webhook/internal"
	"github.com/m198799/timezone-webhook/internal/inject"
	"github.com/m198799/timezone-webhook/internal/log"
)

const (
	jsonContentType = "application/json"
	injectFalse     = "false"
)

var (
	// k8sDecode is define decode factory
	k8sDecode = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

// readAdmissionReview is read http request body decode to v1beta1.AdmissionReview
func (h *RequestsHandler) readAdmissionReview(r *http.Request) (*admission.AdmissionReview, int, error) {
	if r.Method != http.MethodPost {
		log.Error("invalid method,only POST requests are allowed", "method", r.Method)
		return nil, http.StatusMethodNotAllowed, fmt.Errorf("invalid method %s, only POST requests are allowed", r.Method)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("could not read request body, error: %s", err.Error())
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported content type %s, only %s is supported", contentType, jsonContentType)
	}

	review := &admission.AdmissionReview{}
	if _, _, err := k8sDecode.Decode(body, nil, review); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("could not deserialize request to review object: %v", err)
	} else if review.Request == nil {
		return nil, http.StatusBadRequest, errors.New("review parsed but request is null")
	}
	return review, http.StatusOK, nil
}

// handleAdmissionReview is handler admission
func (h *RequestsHandler) handleAdmissionReview(ctx context.Context, review *admission.AdmissionReview) (internal.Patches, error) {
	log.Info(fmt.Sprintf("handleAdmissionReview request is %s namespace %s", review.Request.Kind.String(), review.Request.Namespace))

	if review.Request.Operation == admission.Create &&
		review.Request.Namespace != metav1.NamespaceSystem &&
		review.Request.Namespace != metav1.NamespacePublic &&
		!h.IsFilterNamespace(review.Request.Namespace) {
		return h.handlePodAdmissionRequest(ctx, review.Request)
	}
	return nil, nil
}

// handlePodAdmissionRequest handler pods create reqeust
func (h *RequestsHandler) handlePodAdmissionRequest(ctx context.Context, req *admission.AdmissionRequest) (internal.Patches, error) {
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := k8sDecode.Decode(raw, nil, &pod); err != nil {
		log.Error("could not deserialize pod object", "err", err)
		return nil, fmt.Errorf("could not deserialize pod object: %v", err)
	}
	var (
		err       error
		patches   internal.Patches       // patches object record patch field
		generator *inject.PatchGenerator // generator is generator patches
	)

	if generator, err = h.lookupPod(ctx, req.Namespace, &pod); err != nil {
		return nil, fmt.Errorf("failed to lookup generator, error: %w", err)
	} else if generator == nil {
		return patches, nil
	}

	if patches, err = generator.Generate(ctx, &pod, ""); err != nil {
		return nil, fmt.Errorf("failed to generate patches for pod, error: %w", err)
	}
	return patches, err
}

// lookupPod is from pod or namespace read Annotations()
func (h *RequestsHandler) lookupPod(ctx context.Context, namespace string, pod *corev1.Pod) (*inject.PatchGenerator, error) {
	var (
		err      error
		ok       bool                     // check key whether in the map
		isInject string                   // user set annotation internal.InjectAnnotation value
		timezone string                   // user set annotation value
		strategy inject.InjectionStrategy // user set annotation value
		tmpV     string
		// user set annotation value in namespace
		strategyNamespace inject.InjectionStrategy
		timezoneNamespace string
		injectNamespace   bool
	)
	if h.InjectNamespaceAnnotation {
		injectNamespace, strategyNamespace, timezoneNamespace, err = h.injectNamespace(ctx, namespace)
		if err != nil && injectNamespace {
			return nil, err
		}
	}

	if _, ok = pod.Annotations[internal.InjectedAnnotation]; ok {
		log.Info(fmt.Sprintf("skipping pod (%s/%s) because its already injected", namespace, pod.Name))
		return nil, nil
	}

	// first from pod read,second from namespace read,three from default value
	if isInject, ok = pod.Annotations[internal.InjectAnnotation]; ok {
		if isInject == injectFalse {
			log.Info(fmt.Sprintf("skipping pod (%s/%s) because annotation on pod is explicitly false for injection", namespace, pod.Name))
			return nil, nil
		}
	} else if !h.InjectByDefault {
		log.Info(fmt.Sprintf("skipping pod (%s/%s) because no other instruction and injection disabled by default", namespace, pod.Name))
		return nil, nil
	}

	timezone = h.DefaultTimezone
	if tmpV, ok = pod.Annotations[internal.TimezoneAnnotation]; ok {
		log.Info(fmt.Sprintf("explicit timezone requested on pod's (%s/%s) annotation: %s", namespace, pod.Name, timezone))
		timezone = tmpV
	} else if timezoneNamespace != "" {
		log.Info(fmt.Sprintf("explicit timezone requested on namespace (%s/%s) annotation: %s", namespace, pod.Name, timezone))
		timezone = timezoneNamespace
	}

	strategy = h.DefaultInjectionStrategy
	if tmpV, ok = pod.Annotations[internal.InjectionStrategyAnnotation]; ok {
		strategy = inject.InjectionStrategy(tmpV)
		log.Info(fmt.Sprintf("explicit injection strategy requested on pod's (%s/%s) annotation: %s", namespace, pod.Name, tmpV))
	} else if strategyNamespace != "" {
		strategy = strategyNamespace
		log.Info(fmt.Sprintf("explicit injection strategy requested on namespace (%s/%s) annotation: %s", namespace, pod.Name, tmpV))
	}
	log.Info(fmt.Sprintf("inject.PatchGenerator Strategy is %s,Timezone is %s,ConfigMapName is %s", strategy, timezone, h.ConfigMapName))
	return &inject.PatchGenerator{
		Strategy:       strategy,
		Timezone:       timezone,
		HostPathPrefix: h.HostPathPrefix,
		LocalTimePath:  h.LocalTimePath,
		ConfigMapName:  h.ConfigMapName,
	}, nil
}

func (h *RequestsHandler) injectNamespace(ctx context.Context, namespace string) (bool, inject.InjectionStrategy, string, error) {
	var (
		err          error
		namespaceObj *corev1.Namespace        // read namespace object
		strategy     inject.InjectionStrategy // namespace injection Strategy
		ok           bool                     // check key whether in the map
		isInject     string                   // user set annotation internal.InjectAnnotation value
		timezone     string
		tmpV         string
	)
	if namespaceObj, err = h.clientSet.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
		log.Error("failed to lookup namespace", namespace, "err", err)
		return false, "", "", fmt.Errorf("failed to lookup namespace %s: %v", namespace, err)
	}
	if tmpV, ok = namespaceObj.Annotations[internal.InjectionStrategyAnnotation]; ok {
		strategy = inject.InjectionStrategy(tmpV)
	}

	if tmpV, ok = namespaceObj.Annotations[internal.TimezoneAnnotation]; ok {
		timezone = tmpV
	}
	if isInject, ok = namespaceObj.Annotations[internal.InjectAnnotation]; ok && isInject == injectFalse {
		log.Info(fmt.Sprintf("skipping namespace %s because annotation on namespace is explicitly false for injection", namespace))
		return false, strategy, timezone, nil
	}
	return true, strategy, timezone, nil
}
