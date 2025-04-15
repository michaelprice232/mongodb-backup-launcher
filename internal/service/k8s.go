package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const azWellKnownLabel = "topology.kubernetes.io/zone"

func (s *Service) availabilityZoneToTarget(replicaHostPath string) (string, string, error) {
	parts := strings.Split(replicaHostPath, ".")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("expects the replica host name to be a K8s headless service and have at least 3 domain parts")
	}
	podName := parts[0]
	namespace := parts[2]

	slog.Debug("Finding pod in namespace", "pod", podName, "namespace", namespace)

	// Find the pod and node it is running on
	pod, err := s.conf.K8sClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return "", "", fmt.Errorf("unable to find pod %s in namespace %s based on hostname %s: %w", podName, namespace, replicaHostPath, err)
	}
	if err != nil {
		return "", "", fmt.Errorf("finding pod: %w", err)
	}

	nodeName := pod.Spec.NodeName
	slog.Debug("Pod is running on node", "pod", podName, "node", nodeName)

	// Find the node and which AZ it is in
	node, err := s.conf.K8sClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return "", "", fmt.Errorf("unable to find node %s: %w", nodeName, err)
	}
	if err != nil {
		return "", "", fmt.Errorf("finding node: %w", err)
	}

	az, found := node.Labels[azWellKnownLabel]
	if !found {
		return "", "", fmt.Errorf("unable to find AZ well known label '%s' on node %s", azWellKnownLabel, nodeName)
	}

	slog.Debug("Target AZ", "az", az)
	slog.Debug("Target namespace", "namespace", namespace)

	return az, namespace, nil
}

func (s *Service) createJob(mongoDBHost, az, namespace string) (*batchv1.Job, error) {
	job, err := s.conf.K8sClient.BatchV1().Jobs(namespace).Create(context.Background(), &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "targeted-mongodb-backups-",
			Namespace:    namespace,
			Annotations: map[string]string{
				"created-by": s.conf.Hostname,
			},
			Labels: map[string]string{
				"backup-type": s.conf.BackupType,
				"app":         "mongodb-backups",
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(900),
			BackoffLimit:            pointer.Int32(3),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"karpenter.sh/do-not-disrupt": "true",
					},
					Labels: map[string]string{
						"backup-type": s.conf.BackupType,
						"app":         "mongodb-backups",
					},
				},

				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: "backups",

					Tolerations: []corev1.Toleration{
						{
							Key:      "mongodb-backups",
							Operator: corev1.TolerationOpEqual,
							Value:    "true",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},

					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "topology.kubernetes.io/zone",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{az},
											},
											{
												Key:      "karpenter.sh/nodepool",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"backups"},
											},
										},
									},
								},
							},
						},
					},

					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   s.conf.DockerImageURI,
							Command: []string{"/usr/local/bin/mongodump_k8s.sh", s.conf.BackupType},

							EnvFrom: []corev1.EnvFromSource{
								{ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "backups"},
								}},
							},

							Env: []corev1.EnvVar{
								{
									Name: "MONGO_INITDB_ROOT_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "mongodb"},
											Key:                  "username",
										},
									},
								},
								{
									Name: "MONGO_INITDB_ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "mongodb"},
											Key:                  "password",
										},
									},
								},
								{
									Name:  "MONGO_HOSTLIST",
									Value: mongoDBHost,
								},
							},

							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
									corev1.ResourceCPU:    resource.MustParse("2"),
								},
							},

							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "instance-storage",
									MountPath: "/backups",
								},
							},
						},
					},

					Volumes: []corev1.Volume{
						{
							Name: "instance-storage",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: nil,
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating K8s jobs client: %w", err)
	}

	slog.Debug("Job created", "Job", job.Name)

	return job, nil
}
