package inject

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/m198799/timezone-webhook/internal"
	"github.com/m198799/timezone-webhook/internal/log"
)

// InjectionStrategy ...
type InjectionStrategy string

const (
	// DefaultHostPathPrefix ..
	DefaultHostPathPrefix string = "/usr/share/zoneinfo"
	// DefaultLocalTimePath ...
	DefaultLocalTimePath string = "/etc/localtime"

	// DefaultInjectionStrategy is the default injection strategy of webhook
	DefaultInjectionStrategy = ConfigMapInjectionStrategy
	// ConfigMapInjectionStrategy TODO
	ConfigMapInjectionStrategy InjectionStrategy = "configmap"
	// HostPathInjectionStrategy is an injection strategy where we assume that
	// TZif files exists on the node machines, and we can just mount them
	// with hostPath volumes
	HostPathInjectionStrategy InjectionStrategy = "hostPath"
)

var (
	jsonPointerEscapeReplacer = strings.NewReplacer("~", "~0", "/", "~1")
)

// PatchGenerator ...
type PatchGenerator struct {
	Strategy           InjectionStrategy
	Timezone           string
	InitContainerImage string
	HostPathPrefix     string
	LocalTimePath      string
	ConfigMapName      string
}

// NewPatchGenerator ...
func NewPatchGenerator() PatchGenerator {
	return PatchGenerator{
		Strategy:       DefaultInjectionStrategy,
		Timezone:       internal.DefaultTimezone,
		HostPathPrefix: DefaultHostPathPrefix,
		LocalTimePath:  DefaultLocalTimePath,
		ConfigMapName:  DefaultZoneInfoConfigmapName,
	}
}

// Generate ...
func (g *PatchGenerator) Generate(ctx context.Context, object interface{}, pathPrefix string) (patches internal.Patches, err error) {
	switch o := object.(type) {
	case *appsv1.StatefulSet:
		return g.forPodSpec(&o.Spec.Template.Spec, fmt.Sprintf("%s/spec/template/spec", pathPrefix), map[string]*metav1.ObjectMeta{
			fmt.Sprintf("%s/metadata", pathPrefix):               &o.ObjectMeta,
			fmt.Sprintf("%s/spec/template/metadata", pathPrefix): &o.Spec.Template.ObjectMeta,
		})
	case *appsv1.Deployment:
		return g.forPodSpec(&o.Spec.Template.Spec, fmt.Sprintf("%s/spec/template/spec", pathPrefix), map[string]*metav1.ObjectMeta{
			fmt.Sprintf("%s/metadata", pathPrefix):               &o.ObjectMeta,
			fmt.Sprintf("%s/spec/template/metadata", pathPrefix): &o.Spec.Template.ObjectMeta,
		})
	case *corev1.Pod:
		return g.forPodSpec(&o.Spec, fmt.Sprintf("%s/spec", pathPrefix), map[string]*metav1.ObjectMeta{
			fmt.Sprintf("%s/metadata", pathPrefix): &o.ObjectMeta,
		})
	case *corev1.List:
		return g.handleList(ctx, o, pathPrefix)
	}

	return make(internal.Patches, 0), fmt.Errorf("not injectable object: %T", object)
}

func (g *PatchGenerator) handleList(ctx context.Context, list *corev1.List, pathPrefix string) (internal.Patches, error) {
	var (
		err     error
		patches internal.Patches
		obj     interface{}
		patch   internal.Patches
	)
	if len(list.Items) == 0 {
		return patches, nil
	}

	for i, v := range list.Items {
		if obj, err = parseTypeMetaSkeleton(v.Raw); err != nil {
			return patches, err
		} else if obj == nil {
			continue
		}

		if err = yaml.Unmarshal(v.Raw, obj); err != nil {
			return patches, err
		}

		if patch, err = g.Generate(ctx, obj, fmt.Sprintf("%s/items/%d", pathPrefix, i)); err != nil {
			return patches, err
		}
		patches = append(patches, patch...)
	}

	return patches, nil
}

func (g *PatchGenerator) forPodSpec(spec *corev1.PodSpec, pathPrefix string, postInjectionAnnotations map[string]*metav1.ObjectMeta) (patches internal.Patches, err error) {
	if g.Strategy == HostPathInjectionStrategy {
		patches = append(patches, g.createHostPathPatches(spec, pathPrefix)...)
	} else if g.Strategy == ConfigMapInjectionStrategy {
		patches = append(patches, g.createConfigMapPatches(spec, pathPrefix)...)
	} else {
		return nil, fmt.Errorf("unknown injection strategy specified: %s", g.Strategy)
	}

	patches = append(patches, g.createEnvironmentVariablePatches(spec, pathPrefix)...)

	for k, v := range postInjectionAnnotations {
		patches = append(patches, g.createPostInjectionAnnotations(v, k)...)
	}

	return patches, nil
}

func (g *PatchGenerator) createEnvironmentVariablePatches(spec *corev1.PodSpec, pathPrefix string) internal.Patches {
	var patches = internal.Patches{}
	for containerID, containerSpec := range spec.Containers {
		if len(containerSpec.Env) == 0 {
			patches = append(patches, internal.Patch{
				Op:    "add",
				Path:  fmt.Sprintf("%s/containers/%d/env", pathPrefix, containerID),
				Value: []corev1.EnvVar{},
			})
		}

		patches = append(patches, internal.Patch{
			Op:   "add",
			Path: fmt.Sprintf("%s/containers/%d/env/-", pathPrefix, containerID),
			Value: corev1.EnvVar{
				Name:  "TZ",
				Value: g.Timezone,
			},
		})
	}
	return patches
}

func (g *PatchGenerator) createConfigMapPatches(spec *corev1.PodSpec, pathPrefix string) internal.Patches {
	var patches = internal.Patches{}

	containers := len(spec.Containers)
	if containers == 0 {
		return patches
	}

	if len(spec.Volumes) == 0 {
		patches = append(patches, internal.Patch{
			Op:    "add",
			Path:  fmt.Sprintf("%s/volumes", pathPrefix),
			Value: []corev1.Volume{},
		})
	}

	patches = append(patches, internal.Patch{
		Op:   "add",
		Path: fmt.Sprintf("%s/volumes/-", pathPrefix),
		Value: corev1.Volume{
			Name: DefaultVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: g.ConfigMapName,
					},
				},
			},
		},
	})

	for containerID := 0; containerID < containers; containerID++ {
		if len(spec.Containers[containerID].VolumeMounts) == 0 {
			patches = append(patches, internal.Patch{
				Op:    "add",
				Path:  fmt.Sprintf("%s/containers/%d/volumeMounts", pathPrefix, containerID),
				Value: []corev1.VolumeMount{},
			})
		}
		_, timeZone := filepath.Split(g.Timezone)
		log.Info(fmt.Sprintf("timeZone is %s,g.Timezone is %s", timeZone, g.Timezone))
		patches = append(patches, internal.Patch{
			Op:   "add",
			Path: fmt.Sprintf("%s/containers/%d/volumeMounts/-", pathPrefix, containerID),
			Value: corev1.VolumeMount{
				Name:      DefaultVolumeName,
				ReadOnly:  true,
				MountPath: g.LocalTimePath,
				SubPath:   timeZone,
			},
		})
	}
	// TODO initContainer zoneinfo
	return patches
}

func (g *PatchGenerator) createHostPathPatches(spec *corev1.PodSpec, pathPrefix string) internal.Patches {
	var patches = internal.Patches{}
	containers := len(spec.Containers)
	if containers == 0 {
		return patches
	}

	for containerID := 0; containerID < containers; containerID++ {
		if len(spec.Containers[containerID].VolumeMounts) == 0 {
			patches = append(patches, internal.Patch{
				Op:    "add",
				Path:  fmt.Sprintf("%s/containers/%d/volumeMounts", pathPrefix, containerID),
				Value: []corev1.VolumeMount{},
			})
		}

		patches = append(patches, internal.Patch{
			Op:   "add",
			Path: fmt.Sprintf("%s/containers/%d/volumeMounts/-", pathPrefix, containerID),
			Value: corev1.VolumeMount{
				Name:      "webhook",
				ReadOnly:  true,
				MountPath: g.LocalTimePath,
				SubPath:   g.Timezone,
			},
		})

		patches = append(patches, internal.Patch{
			Op:   "add",
			Path: fmt.Sprintf("%s/containers/%d/volumeMounts/-", pathPrefix, containerID),
			Value: corev1.VolumeMount{
				Name:      "webhook",
				ReadOnly:  true,
				MountPath: DefaultHostPathPrefix,
			},
		})
	}

	if len(spec.Volumes) == 0 {
		patches = append(patches, internal.Patch{
			Op:    "add",
			Path:  fmt.Sprintf("%s/volumes", pathPrefix),
			Value: []corev1.Volume{},
		})
	}

	patches = append(patches, internal.Patch{
		Op:   "add",
		Path: fmt.Sprintf("%s/volumes/-", pathPrefix),
		Value: corev1.Volume{
			Name: "webhook",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: g.HostPathPrefix,
				},
			},
		},
	})

	return patches
}

func (g *PatchGenerator) createPostInjectionAnnotations(meta *metav1.ObjectMeta, pathPrefix string) internal.Patches {
	var patches = internal.Patches{}
	if len(meta.Annotations) == 0 {
		patches = append(patches, internal.Patch{
			Op:    "add",
			Path:  fmt.Sprintf("%s/annotations", pathPrefix),
			Value: map[string]string{},
		})
	}

	patches = append(patches, internal.Patch{
		Op:    "add",
		Path:  fmt.Sprintf("%s/annotations/%s", pathPrefix, escapeJSONPointer(internal.InjectedAnnotation)),
		Value: "true",
	})
	patches = append(patches, internal.Patch{
		Op:    "add",
		Path:  fmt.Sprintf("%s/annotations/%s", pathPrefix, escapeJSONPointer(internal.TimezoneAnnotation)),
		Value: g.Timezone,
	})

	return patches
}

func parseTypeMetaSkeleton(data []byte) (interface{}, error) {
	var meta metav1.TypeMeta
	err := yaml.Unmarshal(data, &meta)
	if err != nil {
		return nil, err
	}

	switch meta.Kind {
	case "StatefulSet":
		return &appsv1.StatefulSet{}, nil
	case "Deployment":
		return &appsv1.Deployment{}, nil
	case "Pod":
		return &corev1.Pod{}, nil
	case "List":
		return &corev1.List{}, nil
	}
	return nil, nil
}

func escapeJSONPointer(p string) string {
	return jsonPointerEscapeReplacer.Replace(p)
}
