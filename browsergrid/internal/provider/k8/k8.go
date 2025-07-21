package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	kstorage "github.com/autocrawlerHQ/browsergrid/internal/storage/k8"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
	"github.com/google/uuid"
)

const (
	defaultNamespace = "browsergrid"
	defaultPort      = 80
)

type Config struct {
	Namespace        string                  `json:"namespace"`
	ServiceAccount   string                  `json:"service_account"`
	ImagePullSecrets []string                `json:"image_pull_secrets"`
	NodeSelector     map[string]string       `json:"node_selector"`
	Tolerations      []corev1.Toleration     `json:"tolerations"`
	SecurityContext  *corev1.SecurityContext `json:"security_context"`
}

type KubernetesProvisioner struct {
	client  kubernetes.Interface
	storage storage.Storage
	config  Config
}

func NewKubernetesProvisioner(config Config, storageConfig kstorage.Config) (*KubernetesProvisioner, error) {
	// Create Kubernetes client
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeConfig, err = rest.NewForConfig(&rest.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create storage instance
	storage, err := kstorage.NewKubernetesStorage(storageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Set defaults
	if config.Namespace == "" {
		config.Namespace = defaultNamespace
	}

	return &KubernetesProvisioner{
		client:  client,
		storage: storage,
		config:  config,
	}, nil
}

func (p *KubernetesProvisioner) GetType() workpool.ProviderType {
	return workpool.ProviderType("kubernetes")
}

func (p *KubernetesProvisioner) Start(ctx context.Context, sess *sessions.Session) (wsURL, liveURL string, err error) {
	shortID := sess.ID.String()[:8]
	log.Printf("[K8S] Starting session %s", shortID)

	// Create storage resource if profile is specified
	var profileResource *storage.Resource
	if sess.ProfileID != nil {
		profileResource = &storage.Resource{
			ID:      sess.ProfileID.String(),
			Type:    storage.ResourceTypeProfile,
			OwnerID: sess.ID.String(), // TODO: Use actual user ID
			Name:    fmt.Sprintf("profile-%s", sess.ProfileID.String()[:8]),
			Labels: map[string]string{
				"browser": string(sess.Browser),
			},
		}

		// Initialize storage
		if err := p.storage.Initialize(ctx, profileResource); err != nil {
			return "", "", fmt.Errorf("failed to initialize profile storage: %w", err)
		}
	}

	// Create the browser job
	job := p.createBrowserJob(sess, profileResource)

	// Create the job
	createdJob, err := p.client.BatchV1().Jobs(p.config.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create job: %w", err)
	}

	// Create a service to expose the browser
	service := p.createBrowserService(sess)
	createdService, err := p.client.CoreV1().Services(p.config.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		// Clean up job
		p.client.BatchV1().Jobs(p.config.Namespace).Delete(ctx, createdJob.Name, metav1.DeleteOptions{})
		return "", "", fmt.Errorf("failed to create service: %w", err)
	}

	// Wait for the pod to be ready
	podName, err := p.waitForPodReady(ctx, sess.ID.String())
	if err != nil {
		// Clean up resources
		p.cleanup(ctx, sess.ID.String())
		return "", "", fmt.Errorf("browser pod failed to start: %w", err)
	}

	// Get the service endpoint
	endpoint := fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, p.config.Namespace)

	// Store pod name for later reference
	sess.ContainerID = &podName

	// Return WebSocket and HTTP endpoints
	wsURL = fmt.Sprintf("ws://%s:%d", endpoint, defaultPort)
	liveURL = fmt.Sprintf("http://%s:%d", endpoint, defaultPort)

	log.Printf("[K8S] ✓ Session %s started successfully", shortID)
	log.Printf("[K8S] └── Pod: %s", podName)
	log.Printf("[K8S] └── WebSocket: %s", wsURL)
	log.Printf("[K8S] └── Live URL: %s", liveURL)

	return wsURL, liveURL, nil
}

func (p *KubernetesProvisioner) Stop(ctx context.Context, sess *sessions.Session) error {
	log.Printf("[K8S] Stopping session %s", sess.ID.String()[:8])
	return p.cleanup(ctx, sess.ID.String())
}

func (p *KubernetesProvisioner) HealthCheck(ctx context.Context, sess *sessions.Session) error {
	if sess.ContainerID == nil {
		return fmt.Errorf("no pod name recorded")
	}

	// Check if pod is still running
	pod, err := p.client.CoreV1().Pods(p.config.Namespace).Get(ctx, *sess.ContainerID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod is not running: %s", pod.Status.Phase)
	}

	// Check if the browser container is ready
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			return fmt.Errorf("pod is not ready: %s", condition.Message)
		}
	}

	return nil
}

func (p *KubernetesProvisioner) GetMetrics(ctx context.Context, sess *sessions.Session) (*sessions.SessionMetrics, error) {
	if sess.ContainerID == nil {
		return nil, fmt.Errorf("no pod name recorded")
	}

	// Get pod metrics using metrics API
	// In a real implementation, you would use the metrics API client
	// For now, return mock metrics

	return &sessions.SessionMetrics{
		ID:             uuid.New(),
		SessionID:      sess.ID,
		Timestamp:      time.Now(),
		CPUPercent:     ptrFloat64(25.5),
		MemoryMB:       ptrFloat64(512.0),
		NetworkRXBytes: ptrInt64(1024 * 1024),
		NetworkTXBytes: ptrInt64(512 * 1024),
	}, nil
}

// Helper methods

func (p *KubernetesProvisioner) createBrowserJob(sess *sessions.Session, profileResource *storage.Resource) *batchv1.Job {
	jobName := fmt.Sprintf("browsergrid-session-%s", sess.ID)
	browserImage := fmt.Sprintf("browsergrid/%s:%s", sess.Browser, defaultString(string(sess.Version), "latest"))

	// Build environment variables
	env := []corev1.EnvVar{
		{Name: "HEADLESS", Value: strconv.FormatBool(sess.Headless)},
		{Name: "RESOLUTION_WIDTH", Value: strconv.Itoa(sess.Screen.Width)},
		{Name: "RESOLUTION_HEIGHT", Value: strconv.Itoa(sess.Screen.Height)},
		{Name: "DISPLAY", Value: ":1"},
		{Name: "VNC_PORT", Value: "5900"},
		{Name: "REMOTE_DEBUGGING_PORT", Value: "61000"},
	}

	// Add custom environment variables
	if sess.Environment != nil {
		var envMap map[string]string
		if err := json.Unmarshal(sess.Environment, &envMap); err == nil {
			for k, v := range envMap {
				env = append(env, corev1.EnvVar{Name: k, Value: v})
			}
		}
	}

	// Build volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "dshm",
			MountPath: "/dev/shm",
		},
	}
	volumes := []corev1.Volume{
		{
			Name: "dshm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumMemory,
					SizeLimit: resource.NewQuantity(2*1024*1024*1024, resource.BinarySI), // 2GB
				},
			},
		},
	}

	// Add profile volume if specified
	if profileResource != nil {
		kstorage := p.storage.(*kstorage.KubernetesStorage)

		// Add volume
		volumes = append(volumes, corev1.Volume{
			Name: "profile",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("browsergrid-%s-%s", profileResource.Type, profileResource.ID),
				},
			},
		})

		// Add volume mount - Chrome expects profile at /home/user/data-dir
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "profile",
			MountPath: "/home/user/data-dir",
			SubPath:   "chrome-profile", // Use subpath to organize data
		})
	}

	// Build resource requirements
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
	}

	// Override with session resource limits if specified
	if sess.ResourceLimits.CPU != nil {
		cpuStr := fmt.Sprintf("%.2f", *sess.ResourceLimits.CPU)
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuStr)
	}
	if sess.ResourceLimits.Memory != nil {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(*sess.ResourceLimits.Memory)
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:            "browser",
				Image:           browserImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env:             env,
				Resources:       resources,
				VolumeMounts:    volumeMounts,
				Ports: []corev1.ContainerPort{
					{ContainerPort: int32(defaultPort), Name: "http"},
					{ContainerPort: 5900, Name: "vnc"},
					{ContainerPort: 61000, Name: "devtools"},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(defaultPort),
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       5,
					TimeoutSeconds:      3,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(defaultPort),
						},
					},
					InitialDelaySeconds: 30,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
			},
		},
		Volumes: volumes,
	}

	// Apply security context
	if p.config.SecurityContext != nil {
		podSpec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser:  p.config.SecurityContext.RunAsUser,
			RunAsGroup: p.config.SecurityContext.RunAsGroup,
			FSGroup:    p.config.SecurityContext.FSGroup,
		}
	}

	// Apply node selector
	if len(p.config.NodeSelector) > 0 {
		podSpec.NodeSelector = p.config.NodeSelector
	}

	// Apply tolerations
	if len(p.config.Tolerations) > 0 {
		podSpec.Tolerations = p.config.Tolerations
	}

	// Apply service account
	if p.config.ServiceAccount != "" {
		podSpec.ServiceAccountName = p.config.ServiceAccount
	}

	// Apply image pull secrets
	if len(p.config.ImagePullSecrets) > 0 {
		for _, secret := range p.config.ImagePullSecrets {
			podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets, corev1.LocalObjectReference{
				Name: secret,
			})
		}
	}

	// Create job
	ttl := int32(3600) // Clean up after 1 hour
	completions := int32(1)
	parallelism := int32(1)
	backoffLimit := int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: p.config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":         "browsergrid",
				"app.kubernetes.io/component":    "browser",
				"browsergrid.io/session-id":      sess.ID.String(),
				"browsergrid.io/browser":         string(sess.Browser),
				"browsergrid.io/browser-version": string(sess.Version),
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Completions:             &completions,
			Parallelism:             &parallelism,
			BackoffLimit:            &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      "browsergrid",
						"app.kubernetes.io/component": "browser",
						"browsergrid.io/session-id":   sess.ID.String(),
						"browsergrid.io/browser":      string(sess.Browser),
					},
				},
				Spec: podSpec,
			},
		},
	}
}

func (p *KubernetesProvisioner) createBrowserService(sess *sessions.Session) *corev1.Service {
	serviceName := fmt.Sprintf("browsergrid-session-%s", sess.ID)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: p.config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "browsergrid",
				"app.kubernetes.io/component": "browser",
				"browsergrid.io/session-id":   sess.ID.String(),
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"browsergrid.io/session-id": sess.ID.String(),
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       int32(defaultPort),
					TargetPort: intstr.FromInt(defaultPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "vnc",
					Port:       5900,
					TargetPort: intstr.FromInt(5900),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

func (p *KubernetesProvisioner) waitForPodReady(ctx context.Context, sessionID string) (string, error) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for pod to be ready")
		case <-ticker.C:
			// List pods for this session
			pods, err := p.client.CoreV1().Pods(p.config.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("browsergrid.io/session-id=%s", sessionID),
			})
			if err != nil {
				return "", err
			}

			if len(pods.Items) == 0 {
				continue // Still waiting for pod to be created
			}

			pod := &pods.Items[0]

			// Check pod status
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
					return pod.Name, nil
				}
			} else if pod.Status.Phase == corev1.PodFailed {
				// Pod failed, check reason
				reason := "unknown"
				if pod.Status.Reason != "" {
					reason = pod.Status.Reason
				}
				return "", fmt.Errorf("pod failed: %s", reason)
			}
		}
	}
}

func (p *KubernetesProvisioner) cleanup(ctx context.Context, sessionID string) error {
	// Delete job
	jobName := fmt.Sprintf("browsergrid-session-%s", sessionID)
	err := p.client.BatchV1().Jobs(p.config.Namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
	})
	if err != nil {
		log.Printf("[K8S] Failed to delete job %s: %v", jobName, err)
	}

	// Delete service
	serviceName := fmt.Sprintf("browsergrid-session-%s", sessionID)
	err = p.client.CoreV1().Services(p.config.Namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("[K8S] Failed to delete service %s: %v", serviceName, err)
	}

	return nil
}

// Helper functions
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }

// Register the provider
func init() {
	// This would be called during initialization
	// provider.Register(workpool.ProviderType("kubernetes"), provisioner)
}
