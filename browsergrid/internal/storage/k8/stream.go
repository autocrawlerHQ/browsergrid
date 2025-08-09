package kubernetes

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
)

// OpenReader returns a reader for resource data using kubectl exec
func (s *KubernetesStorage) OpenReader(ctx context.Context, resourceID string, path string) (io.ReadCloser, error) {
	resource, err := s.Get(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	// Create a helper pod to access the volume
	pod, err := s.createHelperPod(ctx, resource, "reader", true)
	if err != nil {
		return nil, fmt.Errorf("failed to create helper pod: %w", err)
	}

	// Wait for pod to be ready
	if err := s.waitForPodReady(ctx, pod.Name); err != nil {
		s.deleteHelperPod(ctx, pod.Name)
		return nil, fmt.Errorf("helper pod failed to start: %w", err)
	}

	// Create exec command to cat the file
	req := s.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"cat", filepath.Join(s.getMountPath(resource), path)},
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.config.RestConfig, "POST", req.URL())
	if err != nil {
		s.deleteHelperPod(ctx, pod.Name)
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	reader, writer := io.Pipe()

	// Start streaming in background
	go func() {
		defer writer.Close()
		defer s.deleteHelperPod(context.Background(), pod.Name)

		err := exec.Stream(remotecommand.StreamOptions{
			Stdout: writer,
			Stderr: os.Stderr,
		})
		if err != nil {
			writer.CloseWithError(err)
		}
	}()

	return reader, nil
}

// OpenWriter returns a writer for resource data
func (s *KubernetesStorage) OpenWriter(ctx context.Context, resourceID string, path string) (io.WriteCloser, error) {
	resource, err := s.Get(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	// Create a helper pod to access the volume
	pod, err := s.createHelperPod(ctx, resource, "writer", false)
	if err != nil {
		return nil, fmt.Errorf("failed to create helper pod: %w", err)
	}

	// Wait for pod to be ready
	if err := s.waitForPodReady(ctx, pod.Name); err != nil {
		s.deleteHelperPod(ctx, pod.Name)
		return nil, fmt.Errorf("helper pod failed to start: %w", err)
	}

	// Create directory if needed
	dirPath := filepath.Dir(filepath.Join(s.getMountPath(resource), path))
	mkdirReq := s.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"mkdir", "-p", dirPath},
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	mkdirExec, err := remotecommand.NewSPDYExecutor(s.config.RestConfig, "POST", mkdirReq.URL())
	if err == nil {
		mkdirExec.Stream(remotecommand.StreamOptions{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	}

	// Create exec command to write the file
	req := s.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"tee", filepath.Join(s.getMountPath(resource), path)},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.config.RestConfig, "POST", req.URL())
	if err != nil {
		s.deleteHelperPod(ctx, pod.Name)
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	reader, writer := io.Pipe()

	// Start streaming in background
	go func() {
		defer s.deleteHelperPod(context.Background(), pod.Name)

		err := exec.Stream(remotecommand.StreamOptions{
			Stdin:  reader,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		if err != nil {
			reader.CloseWithError(err)
		}
	}()

	return &streamWriter{
		writer: writer,
		cleanup: func() {
			reader.Close()
		},
	}, nil
}

// GetMetadata retrieves resource metadata
func (s *KubernetesStorage) GetMetadata(ctx context.Context, resourceID string) (*storage.Metadata, error) {
	resource, err := s.Get(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	// Get PVC to check size
	pvcName := s.getPVCName(resource)
	pvc, err := s.client.CoreV1().PersistentVolumeClaims(s.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get PVC: %w", err)
	}

	metadata := &storage.Metadata{
		Custom: make(map[string]string),
	}

	// Get size from PVC status
	if pvc.Status.Capacity != nil {
		if quantity, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			metadata.Size = quantity.Value()
		}
	}

	// Copy custom annotations
	for k, v := range pvc.Annotations {
		if !isSystemAnnotation(k) {
			metadata.Custom[k] = v
		}
	}

	return metadata, nil
}

// UpdateMetadata updates resource metadata
func (s *KubernetesStorage) UpdateMetadata(ctx context.Context, resourceID string, metadata *storage.Metadata) error {
	resource, err := s.Get(ctx, resourceID)
	if err != nil {
		return err
	}

	pvcName := s.getPVCName(resource)
	pvc, err := s.client.CoreV1().PersistentVolumeClaims(s.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	// Update annotations
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}

	for k, v := range metadata.Custom {
		pvc.Annotations[k] = v
	}

	_, err = s.client.CoreV1().PersistentVolumeClaims(s.namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update PVC: %w", err)
	}

	return nil
}

// GetUsage returns storage usage statistics
func (s *KubernetesStorage) GetUsage(ctx context.Context, resourceID string) (*storage.Usage, error) {
	resource, err := s.Get(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	// Create a job to calculate usage
	job := s.createUsageJob(resource)

	_, err = s.client.BatchV1().Jobs(s.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create usage job: %w", err)
	}
	defer s.client.BatchV1().Jobs(s.namespace).Delete(ctx, job.Name, metav1.DeleteOptions{})

	// Wait for job to complete
	usage, err := s.waitForUsageJob(ctx, job.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}

	return usage, nil
}

// Helper methods

func (s *KubernetesStorage) createHelperPod(ctx context.Context, resource *storage.Resource, suffix string, readOnly bool) (*corev1.Pod, error) {
	podName := fmt.Sprintf("browsergrid-storage-%s-%s-%s", resource.Type, resource.ID, suffix)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "browsergrid",
				"app.kubernetes.io/component": "storage-helper",
				"browsergrid.io/resource-id":  resource.ID,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "helper",
					Image:   "busybox:latest",
					Command: []string{"sleep", "3600"}, // Keep alive for 1 hour
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "storage",
							MountPath: s.getMountPath(resource),
							ReadOnly:  readOnly,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: s.getPVCName(resource),
							ReadOnly:  readOnly,
						},
					},
				},
			},
		},
	}

	return s.client.CoreV1().Pods(s.namespace).Create(ctx, pod, metav1.CreateOptions{})
}

func (s *KubernetesStorage) createUsageJob(resource *storage.Resource) *batchv1.Job {
	jobName := fmt.Sprintf("browsergrid-usage-%s-%s", resource.Type, resource.ID)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "browsergrid",
				"app.kubernetes.io/component": "storage-usage",
				"browsergrid.io/resource-id":  resource.ID,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "usage",
							Image: "busybox:latest",
							Command: []string{
								"sh", "-c",
								fmt.Sprintf("du -sb %s | awk '{print $1}' > /tmp/usage.txt && find %s -type f | wc -l > /tmp/count.txt",
									s.getMountPath(resource), s.getMountPath(resource)),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "storage",
									MountPath: s.getMountPath(resource),
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: s.getPVCName(resource),
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (s *KubernetesStorage) waitForPodReady(ctx context.Context, podName string) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod to be ready")
		case <-ticker.C:
			pod, err := s.client.CoreV1().Pods(s.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if pod.Status.Phase == corev1.PodRunning {
				// Check if all containers are ready
				ready := true
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
						ready = false
						break
					}
				}
				if ready {
					return nil
				}
			}
		}
	}
}

func (s *KubernetesStorage) waitForUsageJob(ctx context.Context, jobName string) (*storage.Usage, error) {
	// Wait for job to complete
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for usage job")
		case <-ticker.C:
			job, err := s.client.BatchV1().Jobs(s.namespace).Get(ctx, jobName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			if job.Status.Succeeded > 0 {
				// Job completed, extract results
				// In a real implementation, we'd read the output from the pod logs
				// For now, return mock data
				return &storage.Usage{
					BytesUsed:  1024 * 1024, // 1MB
					FilesCount: 10,
				}, nil
			}
			if job.Status.Failed > 0 {
				return nil, fmt.Errorf("usage job failed")
			}
		}
	}
}

func (s *KubernetesStorage) deleteHelperPod(ctx context.Context, podName string) error {
	return s.client.CoreV1().Pods(s.namespace).Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // Delete immediately
	})
}

func isSystemAnnotation(key string) bool {
	return key == "browsergrid.io/resource-id" ||
		key == "browsergrid.io/resource-type" ||
		key == "browsergrid.io/owner-id" ||
		key == "browsergrid.io/created-at"
}

// streamWriter wraps an io.WriteCloser with cleanup
type streamWriter struct {
	writer  io.WriteCloser
	cleanup func()
}

func (w *streamWriter) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

func (w *streamWriter) Close() error {
	err := w.writer.Close()
	if w.cleanup != nil {
		w.cleanup()
	}
	return err
}
