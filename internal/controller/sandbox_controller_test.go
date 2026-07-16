/*
Copyright 2026.

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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/controller/podspec"
)

var sandboxCounter int

// createTemplate creates a minimal SandboxTemplate.
func createTemplate(name string) {
	tpl := &kubeparkv1alpha1.SandboxTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kubeparkv1alpha1.SandboxTemplateSpec{
			Image:    "ghcr.io/example/ops:latest",
			HomeSize: resource.MustParse("1Gi"),
		},
	}
	Expect(k8sClient.Create(ctx, tpl)).To(Succeed())
}

// newSandbox returns an unsaved Sandbox in the default namespace.
func newSandbox(template string) *kubeparkv1alpha1.Sandbox {
	sandboxCounter++
	return &kubeparkv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("sb-%d", sandboxCounter),
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kubeparkv1alpha1.SandboxSpec{
			Template: template,
			Owner:    kubeparkv1alpha1.OwnerSpec{Name: "alice@example.com"},
		},
	}
}

// markPodReady flips an envtest pod (which never actually runs) to Running
// and Ready so the controller can observe a live pod.
func markPodReady(name string) {
	var pod corev1.Pod
	Eventually(func() error {
		return k8sClient.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: name}, &pod)
	}, 10*time.Second, 200*time.Millisecond).Should(Succeed())

	pod.Status.Phase = corev1.PodRunning
	pod.Status.PodIP = "10.1.2.3"
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	Expect(k8sClient.Status().Update(ctx, &pod)).To(Succeed())
}

func phaseOf(name string) kubeparkv1alpha1.SandboxPhase {
	var sb kubeparkv1alpha1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: name}, &sb); err != nil {
		return ""
	}
	return sb.Status.Phase
}

var _ = Describe("Sandbox Controller", func() {
	Context("provisioning a sandbox", func() {
		It("creates PVC, host key, network policy and pod, and reaches Running", func() {
			createTemplate("tpl-run")
			sb := newSandbox("tpl-run")
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())

			By("creating the home PVC")
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: podspec.PVCName(sb.Name)}, &pvc)
			}, 10*time.Second, 200*time.Millisecond).Should(Succeed())

			By("creating the host key secret")
			Eventually(func() error {
				var s corev1.Secret
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: podspec.HostKeyName(sb.Name)}, &s)
			}, 10*time.Second, 200*time.Millisecond).Should(Succeed())

			By("creating a default-deny network policy")
			Eventually(func() error {
				var np networkingv1.NetworkPolicy
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: podspec.NetPolName(sb.Name)}, &np)
			}, 10*time.Second, 200*time.Millisecond).Should(Succeed())

			By("creating the pod and reaching Running once the pod is ready")
			markPodReady(podspec.PodName(sb.Name))
			Eventually(func() kubeparkv1alpha1.SandboxPhase {
				return phaseOf(sb.Name)
			}, 15*time.Second, 300*time.Millisecond).Should(Equal(kubeparkv1alpha1.SandboxPhaseRunning))

			By("recording the pod IP and initializing the idle clock")
			var got kubeparkv1alpha1.Sandbox
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: sb.Name}, &got)).To(Succeed())
			Expect(got.Status.PodIP).To(Equal("10.1.2.3"))
			Expect(got.Status.LastActivityTime).NotTo(BeNil())
		})
	})

	Context("suspending a sandbox", func() {
		It("deletes the pod but keeps the PVC when desiredState is Stopped", func() {
			createTemplate("tpl-suspend")
			sb := newSandbox("tpl-suspend")
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())
			markPodReady(podspec.PodName(sb.Name))
			Eventually(func() kubeparkv1alpha1.SandboxPhase {
				return phaseOf(sb.Name)
			}, 15*time.Second, 300*time.Millisecond).Should(Equal(kubeparkv1alpha1.SandboxPhaseRunning))

			By("stopping the sandbox")
			var got kubeparkv1alpha1.Sandbox
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: sb.Name}, &got)).To(Succeed())
			got.Spec.DesiredState = kubeparkv1alpha1.DesiredStateStopped
			Expect(k8sClient.Update(ctx, &got)).To(Succeed())

			By("deleting the pod")
			Eventually(func() bool {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: podspec.PodName(sb.Name)}, &pod)
				return apierrors.IsNotFound(err) || !pod.DeletionTimestamp.IsZero()
			}, 15*time.Second, 300*time.Millisecond).Should(BeTrue())

			By("keeping the PVC")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Namespace: sb.Namespace, Name: podspec.PVCName(sb.Name)}, &pvc)).To(Succeed())
		})
	})

	Context("deleting a sandbox", func() {
		It("retains the home PVC by default and orphans it", func() {
			createTemplate("tpl-del")
			sb := newSandbox("tpl-del")
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: podspec.PVCName(sb.Name)}, &pvc)
			}, 10*time.Second, 200*time.Millisecond).Should(Succeed())

			By("deleting the sandbox")
			Expect(k8sClient.Delete(ctx, sb)).To(Succeed())
			Eventually(func() bool {
				var got kubeparkv1alpha1.Sandbox
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: sb.Name}, &got)
				return apierrors.IsNotFound(err)
			}, 15*time.Second, 300*time.Millisecond).Should(BeTrue())

			By("keeping the PVC, labeled as an orphaned home")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Namespace: sb.Namespace, Name: podspec.PVCName(sb.Name)}, &pvc)).To(Succeed())
			Expect(pvc.Labels).To(HaveKeyWithValue(LabelOrphanedHome, "true"))
		})
	})

	Context("invalid template reference", func() {
		It("stays Pending with Ready=False/InvalidRef", func() {
			sb := newSandbox("does-not-exist")
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())
			Eventually(func() string {
				var got kubeparkv1alpha1.Sandbox
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: sb.Name}, &got); err != nil {
					return ""
				}
				cond := meta.FindStatusCondition(got.Status.Conditions, kubeparkv1alpha1.ConditionReady)
				if cond == nil {
					return ""
				}
				return cond.Reason
			}, 15*time.Second, 300*time.Millisecond).Should(Equal(kubeparkv1alpha1.ReasonInvalidRef))
			Expect(phaseOf(sb.Name)).To(Equal(kubeparkv1alpha1.SandboxPhasePending))
		})
	})
})
