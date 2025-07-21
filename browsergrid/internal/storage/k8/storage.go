package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
)

type Config struct {
	Namespace     string            `json:"namespace"`
	StorageClass  string            `json:"storage_class"`
	VolumeSize    string            `json:"volume_size"`
	AccessMode    string            `json:"access_mode"`
	Labels        map[string]string `json:"labels"`
	ReclaimPolicy string            `json:"reclaim_policy"`
}

type KubernetesStorage struct {
	client    kubernetes.Interface
	config    Config
	namespace string
}

func NewKubernetesStorage(config Config) (*KubernetesStorage, error) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		kubeConfig, err = rest.NewForConfig(&rest.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	if config.Namespace == "" {
		config.Namespace = "browsergrid"
	}
	if config.VolumeSize == "" {
		config.VolumeSize = "10Gi"
	}
	if config.AccessMode == "" {
		config.AccessMode = "ReadWriteMany"
	}

	return &KubernetesStorage{
		client:    client,
		config:    config,
		namespace: config.Namespace,
	}, nil
}

func (s *KubernetesStorage) Initialize(ctx context.Context, resource *storage.Resource) error {
	pvcName := s.getPVCName(resource)

	_, err := s.client.CoreV1().PersistentVolumeClaims(s.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: s.namespace,
			Labels:    s.getLabels(resource),
			Annotations: map[string]string{
				"browsergrid.io/resource-id":   resource.ID,
				"browsergrid.io/resource-type": string(resource.Type),
				"browsergrid.io/owner-id":      resource.OwnerID,
				"browsergrid.io/created-at":    resource.CreatedAt.Format(time.RFC3339),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				s.getAccessMode(),
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(s.config.VolumeSize),
				},
			},
		},
	}

	if s.config.StorageClass != "" {
		pvc.Spec.StorageClassName = &s.config.StorageClass
	}

	_, err = s.client.CoreV1().PersistentVolumeClaims(s.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	return s.waitForPVCReady(ctx, pvcName)
}

func (s *KubernetesStorage) Get(ctx context.Context, resourceID string) (*storage.Resource, error) {
	labelSelector := fmt.Sprintf("browsergrid.io/resource-id=%s", resourceID)
	pvcs, err := s.client.CoreV1().PersistentVolumeClaims(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	if len(pvcs.Items) == 0 {
		return nil, fmt.Errorf("resource not found: %s", resourceID)
	}

	pvc := pvcs.Items[0]
	return s.pvcToResource(&pvc), nil
}

func (s *KubernetesStorage) List(ctx context.Context, filter *storage.ResourceFilter) ([]*storage.Resource, error) {
	listOptions := metav1.ListOptions{}

	var selectors []string
	if filter.Type != nil {
		selectors = append(selectors, fmt.Sprintf("browsergrid.io/resource-type=%s", *filter.Type))
	}
	if filter.OwnerID != nil {
		selectors = append(selectors, fmt.Sprintf("browsergrid.io/owner-id=%s", *filter.OwnerID))
	}
	for k, v := range filter.Labels {
		selectors = append(selectors, fmt.Sprintf("%s=%s", k, v))
	}

	if len(selectors) > 0 {
		listOptions.LabelSelector = joinSelectors(selectors)
	}

	pvcs, err := s.client.CoreV1().PersistentVolumeClaims(s.namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	resources := make([]*storage.Resource, 0, len(pvcs.Items))
	for _, pvc := range pvcs.Items {
		resources = append(resources, s.pvcToResource(&pvc))
	}

	if filter.Offset > 0 && filter.Offset < len(resources) {
		resources = resources[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(resources) {
		resources = resources[:filter.Limit]
	}

	return resources, nil
}

func (s *KubernetesStorage) Delete(ctx context.Context, resourceID string) error {
	resource, err := s.Get(ctx, resourceID)
	if err != nil {
		return err
	}

	pvcName := s.getPVCName(resource)
	err = s.client.CoreV1().PersistentVolumeClaims(s.namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete PVC: %w", err)
	}

	return nil
}

func (s *KubernetesStorage) GetVolumeMount(resource *storage.Resource, readOnly bool) storage.VolumeMount {
	return storage.VolumeMount{
		Name:      s.getPVCName(resource),
		MountPath: s.getMountPath(resource),
		ReadOnly:  readOnly,
	}
}

func (s *KubernetesStorage) GetPodVolumes(resources []*storage.Resource) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(resources))

	for _, resource := range resources {
		pvcName := s.getPVCName(resource)
		volumes = append(volumes, corev1.Volume{
			Name: pvcName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}

	return volumes
}

func (s *KubernetesStorage) GetPodVolumeMounts(resources []*storage.Resource, readOnly bool) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(resources))

	for _, resource := range resources {
		mount := s.GetVolumeMount(resource, readOnly)
		mounts = append(mounts, corev1.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
			SubPath:   mount.SubPath,
		})
	}

	return mounts
}

func (s *KubernetesStorage) getPVCName(resource *storage.Resource) string {
	return fmt.Sprintf("browsergrid-%s-%s", resource.Type, resource.ID)
}

func (s *KubernetesStorage) getMountPath(resource *storage.Resource) string {
	return filepath.Join("/storage", string(resource.Type), resource.ID)
}

func (s *KubernetesStorage) getLabels(resource *storage.Resource) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":       "browsergrid",
		"app.kubernetes.io/component":  "storage",
		"browsergrid.io/resource-id":   resource.ID,
		"browsergrid.io/resource-type": string(resource.Type),
		"browsergrid.io/owner-id":      resource.OwnerID,
	}

	for k, v := range resource.Labels {
		labels[k] = v
	}

	for k, v := range s.config.Labels {
		labels[k] = v
	}

	return labels
}

func (s *KubernetesStorage) getAccessMode() corev1.PersistentVolumeAccessMode {
	switch s.config.AccessMode {
	case "ReadWriteOnce":
		return corev1.ReadWriteOnce
	case "ReadOnlyMany":
		return corev1.ReadOnlyMany
	case "ReadWriteMany":
		return corev1.ReadWriteMany
	default:
		return corev1.ReadWriteMany
	}
}

func (s *KubernetesStorage) waitForPVCReady(ctx context.Context, pvcName string) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for PVC to be ready")
		case <-ticker.C:
			pvc, err := s.client.CoreV1().PersistentVolumeClaims(s.namespace).Get(ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if pvc.Status.Phase == corev1.ClaimBound {
				return nil
			}
		}
	}
}

func (s *KubernetesStorage) pvcToResource(pvc *corev1.PersistentVolumeClaim) *storage.Resource {
	resource := &storage.Resource{
		ID:      pvc.Annotations["browsergrid.io/resource-id"],
		Type:    storage.ResourceType(pvc.Annotations["browsergrid.io/resource-type"]),
		OwnerID: pvc.Annotations["browsergrid.io/owner-id"],
		Name:    pvc.Name,
		Labels:  make(map[string]string),
	}

	if createdStr := pvc.Annotations["browsergrid.io/created-at"]; createdStr != "" {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			resource.CreatedAt = t
		}
	}

	for k, v := range pvc.Labels {
		if !isSystemLabel(k) {
			resource.Labels[k] = v
		}
	}

	return resource
}

func isSystemLabel(key string) bool {
	return key == "app.kubernetes.io/name" ||
		key == "app.kubernetes.io/component" ||
		key == "browsergrid.io/resource-id" ||
		key == "browsergrid.io/resource-type" ||
		key == "browsergrid.io/owner-id"
}

func joinSelectors(selectors []string) string {
	result := ""
	for i, s := range selectors {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
