// Package inject ...
package inject

import (
	"context"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/m198799/timezone-webhook/internal/log"
)

// 在初始化启动时，通过webhook机制往pod中添加volume，挂载在configmap中配置的时区信息。
const (
	// DefaultZoneInfoConfigmapName is default  configmap name for zoneinfo file name
	DefaultZoneInfoConfigmapName string = "im.zoneinfo.configmap.name"

	// DefaultZoneInfoName is default configmap content for utc zoneinfo, read zoneinfo/Shannhai file
	DefaultZoneInfoName string = "UTC"

	// DefaultZoneInfoDir is default zoneinfo dir
	DefaultZoneInfoDir string = "./zoneinfo/"

	// DefaultNamespace is infra-system namespace
	DefaultNamespace string = "juggleim"

	// DefaultVolumeName is default volume name
	DefaultVolumeName = "zoneinfo-configmap"
)

// InitZoneInfoConfigmap first check configmap is existed,second not existed from zoneinfo/Shanghai read conteng create ConfigMap
func InitZoneInfoConfigmap(ctx context.Context, clientSet kubernetes.Interface, ConfigMapName string, Namespaces []string) error {
	var (
		cfg       *v1.ConfigMap
		err       error
		configMap *v1.ConfigMap
	)

	for _, ns := range Namespaces {
		if cfg, err = clientSet.CoreV1().ConfigMaps(ns).Get(ctx, ConfigMapName, metav1.GetOptions{}); err != nil && !errors.IsNotFound(err) {
			log.Error("configmap not find", "namespace", ns, "name", ConfigMapName)
			return err
		} else if cfg != nil && cfg.Name != "" {
			log.Info("for ns configmap is exist", "ns", ns)
			continue
		} else if errors.IsNotFound(err) {
			// Create
			if configMap, err = GenerateZoneInfoConfigmap(ConfigMapName, ns); err != nil {
				return err
			}

			if _, err = clientSet.CoreV1().ConfigMaps(ns).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
				log.Error("Create Configmap error", "err", err)
				return err
			}
		}
	}
	return nil
}

// GenerateZoneInfoConfigmap ...
func GenerateZoneInfoConfigmap(ConfigMapName, NameSpace string) (*v1.ConfigMap, error) {
	var (
		err     error
		entries []os.DirEntry
	)
	if entries, err = os.ReadDir(DefaultZoneInfoDir); err != nil {
		log.Error("Read ZoneInfo from dir error", "dir", DefaultZoneInfoDir)
		return nil, err
	}

	zoneInfoMap := make(map[string][]byte)
	for _, e := range entries {
		zoneInfoFilePath := filepath.Join(DefaultZoneInfoDir, e.Name())
		zoneInfoData, err := os.ReadFile(zoneInfoFilePath)
		if err != nil {
			panic(err)
		}
		zoneInfoMap[e.Name()] = zoneInfoData
	}

	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Configmap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: NameSpace,
		},
		BinaryData: zoneInfoMap,
	}
	return configMap, nil
}
