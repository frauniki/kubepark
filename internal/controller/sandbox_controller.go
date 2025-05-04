/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

const (
	defaultSandboxImage                  = "kubepark/sandbox-ssh:latest"
	defaultTerminationGracePeriodSeconds = int64(30)
	sshConfigVolumeName                  = "ssh-config"
	sshConfigMountPath                   = "/etc/ssh"
	sshPublicKeyConfigMapName            = "ssh-public-key"
	sshAuthorizedKeysPath                = "/etc/ssh/authorized_keys"
	sandboxFinalizer                     = "kubepark.sinoa.jp/sandbox-finalizer"
)

// SandboxReconciler reconciles a Sandbox object
type SandboxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubepark.sinoa.jp,resources=sandboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepark.sinoa.jp,resources=sandboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubepark.sinoa.jp,resources=sandboxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubepark.sinoa.jp,resources=sandboxtemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SandboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Sandbox", "namespace", req.Namespace, "name", req.Name)

	sandbox := &kubeparkv1alpha1.Sandbox{}
	err := r.Get(ctx, req.NamespacedName, sandbox)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Sandbox resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Sandbox")
		return ctrl.Result{}, err
	}

	if sandbox.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(sandbox, sandboxFinalizer) {
			if err := r.finalizeSandbox(ctx, sandbox); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(sandbox, sandboxFinalizer)
			err = r.Update(ctx, sandbox)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(sandbox, sandboxFinalizer) {
		controllerutil.AddFinalizer(sandbox, sandboxFinalizer)
		err = r.Update(ctx, sandbox)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if sandbox.Spec.SandboxTemplateRef != nil {
		if err := r.applySandboxTemplate(ctx, sandbox); err != nil {
			log.Error(err, "Failed to apply SandboxTemplate")
			return ctrl.Result{}, err
		}
	}

	if err := r.reconcileSSHPublicKeyConfigMap(ctx, sandbox); err != nil {
		log.Error(err, "Failed to reconcile SSH public key ConfigMap")
		return ctrl.Result{}, err
	}

	if err := r.reconcileSandboxPod(ctx, sandbox); err != nil {
		log.Error(err, "Failed to reconcile sandbox pod")
		return ctrl.Result{}, err
	}

	if err := r.updateSandboxStatus(ctx, sandbox); err != nil {
		log.Error(err, "Failed to update Sandbox status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SandboxReconciler) finalizeSandbox(ctx context.Context, sandbox *kubeparkv1alpha1.Sandbox) error {
	log := logf.FromContext(ctx)
	log.Info("Finalizing sandbox", "namespace", sandbox.Namespace, "name", sandbox.Name)

	terminationGracePeriod := defaultTerminationGracePeriodSeconds
	if sandbox.Spec.TerminationGracePeriodSeconds != nil {
		terminationGracePeriod = *sandbox.Spec.TerminationGracePeriodSeconds
	}

	log.Info("Waiting for termination grace period", "seconds", terminationGracePeriod)
	time.Sleep(time.Duration(terminationGracePeriod) * time.Second)

	pod := &corev1.Pod{}
	podName := fmt.Sprintf("sandbox-%s", sandbox.Name)
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: sandbox.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := r.Delete(ctx, pod); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	configMap := &corev1.ConfigMap{}
	configMapName := fmt.Sprintf("%s-%s", sshPublicKeyConfigMapName, sandbox.Name)
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: sandbox.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := r.Delete(ctx, configMap); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (r *SandboxReconciler) applySandboxTemplate(ctx context.Context, sandbox *kubeparkv1alpha1.Sandbox) error {
	log := logf.FromContext(ctx)
	log.Info("Applying SandboxTemplate", "template", sandbox.Spec.SandboxTemplateRef.Name)

	template := &kubeparkv1alpha1.SandboxTemplate{}
	err := r.Get(ctx, types.NamespacedName{Name: sandbox.Spec.SandboxTemplateRef.Name, Namespace: sandbox.Namespace}, template)
	if err != nil {
		return err
	}

	if sandbox.Spec.ServiceAccountName == "" {
		sandbox.Spec.ServiceAccountName = template.Spec.ServiceAccountName
	}
	if sandbox.Spec.NodeSelector == nil {
		sandbox.Spec.NodeSelector = template.Spec.NodeSelector
	}
	if sandbox.Spec.Affinity == nil {
		sandbox.Spec.Affinity = template.Spec.Affinity
	}
	if sandbox.Spec.Tolerations == nil {
		sandbox.Spec.Tolerations = template.Spec.Tolerations
	}
	if sandbox.Spec.ImagePullSecrets == nil {
		sandbox.Spec.ImagePullSecrets = template.Spec.ImagePullSecrets
	}
	if sandbox.Spec.HostNetwork == nil {
		sandbox.Spec.HostNetwork = template.Spec.HostNetwork
	}
	if sandbox.Spec.Container == nil {
		sandbox.Spec.Container = template.Spec.Container
	}
	if sandbox.Spec.Image == "" && template.Spec.Image != "" {
		sandbox.Spec.Image = template.Spec.Image
	}
	if sandbox.Spec.TerminationGracePeriodSeconds == nil && template.Spec.TerminationGracePeriodSeconds != nil {
		sandbox.Spec.TerminationGracePeriodSeconds = template.Spec.TerminationGracePeriodSeconds
	}

	return nil
}

func (r *SandboxReconciler) reconcileSSHPublicKeyConfigMap(ctx context.Context, sandbox *kubeparkv1alpha1.Sandbox) error {
	log := logf.FromContext(ctx)
	log.Info("Reconciling SSH public key ConfigMap", "namespace", sandbox.Namespace, "name", sandbox.Name)

	if sandbox.Spec.SSH == nil || sandbox.Spec.SSH.PublicKey == "" {
		return fmt.Errorf("SSH public key is required")
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", sshPublicKeyConfigMapName, sandbox.Name),
			Namespace: sandbox.Namespace,
		},
		Data: map[string]string{
			"authorized_keys": sandbox.Spec.SSH.PublicKey,
		},
	}

	if err := controllerutil.SetControllerReference(sandbox, configMap, r.Scheme); err != nil {
		return err
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, configMap); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		existingConfigMap.Data = configMap.Data
		if err := r.Update(ctx, existingConfigMap); err != nil {
			return err
		}
	}

	return nil
}

func (r *SandboxReconciler) reconcileSandboxPod(ctx context.Context, sandbox *kubeparkv1alpha1.Sandbox) error {
	log := logf.FromContext(ctx)
	log.Info("Reconciling sandbox pod", "namespace", sandbox.Namespace, "name", sandbox.Name)

	image := defaultSandboxImage
	if sandbox.Spec.Image != "" {
		image = sandbox.Spec.Image
	} else if sandbox.Spec.Container != nil && sandbox.Spec.Container.Image != "" {
		image = sandbox.Spec.Container.Image
	}

	terminationGracePeriod := defaultTerminationGracePeriodSeconds
	if sandbox.Spec.TerminationGracePeriodSeconds != nil {
		terminationGracePeriod = *sandbox.Spec.TerminationGracePeriodSeconds
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("sandbox-%s", sandbox.Name),
			Namespace: sandbox.Namespace,
			Labels: map[string]string{
				"app":                 "kubepark",
				"kubepark.sinoa.jp/sandbox": sandbox.Name,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName:            sandbox.Spec.ServiceAccountName,
			NodeSelector:                  sandbox.Spec.NodeSelector,
			Affinity:                      sandbox.Spec.Affinity,
			Tolerations:                   sandbox.Spec.Tolerations,
			ImagePullSecrets:              sandbox.Spec.ImagePullSecrets,
			TerminationGracePeriodSeconds: &terminationGracePeriod,
			Containers: []corev1.Container{
				{
					Name:  "sandbox",
					Image: image,
					Ports: []corev1.ContainerPort{
						{
							Name:          "ssh",
							ContainerPort: 22,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      sshConfigVolumeName,
							MountPath: sshConfigMountPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: sshConfigVolumeName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: fmt.Sprintf("%s-%s", sshPublicKeyConfigMapName, sandbox.Name),
							},
							Items: []corev1.KeyToPath{
								{
									Key:  "authorized_keys",
									Path: "authorized_keys",
								},
							},
						},
					},
				},
			},
		},
	}

	if sandbox.Spec.Container != nil {
		container := &pod.Spec.Containers[0]
		if sandbox.Spec.Container.Resources.Limits != nil {
			container.Resources.Limits = sandbox.Spec.Container.Resources.Limits
		}
		if sandbox.Spec.Container.Resources.Requests != nil {
			container.Resources.Requests = sandbox.Spec.Container.Resources.Requests
		}
		if len(sandbox.Spec.Container.Env) > 0 {
			container.Env = append(container.Env, sandbox.Spec.Container.Env...)
		}
		if len(sandbox.Spec.Container.EnvFrom) > 0 {
			container.EnvFrom = append(container.EnvFrom, sandbox.Spec.Container.EnvFrom...)
		}
		if len(sandbox.Spec.Container.VolumeMounts) > 0 {
			container.VolumeMounts = append(container.VolumeMounts, sandbox.Spec.Container.VolumeMounts...)
		}
		if sandbox.Spec.Container.SecurityContext != nil {
			container.SecurityContext = sandbox.Spec.Container.SecurityContext
		}
	}

	if sandbox.Spec.HostNetwork != nil {
		pod.Spec.HostNetwork = *sandbox.Spec.HostNetwork
	}

	if err := controllerutil.SetControllerReference(sandbox, pod, r.Scheme); err != nil {
		return err
	}

	existingPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, existingPod)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, pod); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if !podSpecEqual(existingPod, pod) {
			if err := r.Delete(ctx, existingPod); err != nil {
				return err
			}
			if err := r.Create(ctx, pod); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *SandboxReconciler) updateSandboxStatus(ctx context.Context, sandbox *kubeparkv1alpha1.Sandbox) error {
	log := logf.FromContext(ctx)
	log.Info("Updating Sandbox status", "namespace", sandbox.Namespace, "name", sandbox.Name)

	pod := &corev1.Pod{}
	podName := fmt.Sprintf("sandbox-%s", sandbox.Name)
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: sandbox.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			sandbox.Status.Phase = kubeparkv1alpha1.SandboxPending
			sandbox.Status.Message = "Sandbox pod not found"
			return r.Status().Update(ctx, sandbox)
		}
		return err
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		sandbox.Status.Phase = kubeparkv1alpha1.SandboxPending
		sandbox.Status.Message = "Sandbox pod is pending"
	case corev1.PodRunning:
		sandbox.Status.Phase = kubeparkv1alpha1.SnadboxRunning
		sandbox.Status.Message = "Sandbox pod is running"
		if sandbox.Status.StartedAt == nil {
			now := metav1.Now()
			sandbox.Status.StartedAt = &now
		}
	case corev1.PodSucceeded:
		sandbox.Status.Phase = kubeparkv1alpha1.SandboxCompleted
		sandbox.Status.Message = "Sandbox pod completed successfully"
		if sandbox.Status.FinishedAt == nil {
			now := metav1.Now()
			sandbox.Status.FinishedAt = &now
		}
	case corev1.PodFailed:
		sandbox.Status.Phase = kubeparkv1alpha1.SandboxFailed
		sandbox.Status.Message = "Sandbox pod failed"
		if sandbox.Status.FinishedAt == nil {
			now := metav1.Now()
			sandbox.Status.FinishedAt = &now
		}
	default:
		sandbox.Status.Phase = kubeparkv1alpha1.SandboxUnknown
		sandbox.Status.Message = fmt.Sprintf("Unknown pod phase: %s", pod.Status.Phase)
	}

	return r.Status().Update(ctx, sandbox)
}

func podSpecEqual(pod1, pod2 *corev1.Pod) bool {
	return pod1.Spec.ServiceAccountName == pod2.Spec.ServiceAccountName &&
		pod1.Spec.HostNetwork == pod2.Spec.HostNetwork
}

// SetupWithManager sets up the controller with the Manager.
func (r *SandboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeparkv1alpha1.Sandbox{}).
		Named("sandbox").
		Complete(r)
}
