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
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/controller/podspec"
)

var profileCounter int

func createNamespace(name string) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := k8sClient.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func createProfile(name, allowedNS string, grantNS ...string) {
	profileCounter++
	profile := &kubeparkv1alpha1.AccessProfile{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kubeparkv1alpha1.AccessProfileSpec{
			AllowedNamespaces: []string{allowedNS},
			Grants: []kubeparkv1alpha1.NamespacedGrant{{
				Namespaces: grantNS,
				Rules: []rbacv1.PolicyRule{{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list"},
				}},
			}},
		},
	}
	Expect(k8sClient.Create(ctx, profile)).To(Succeed())
}

func rbacReadyReason(name string) string {
	var sb kubeparkv1alpha1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: name}, &sb); err != nil {
		return ""
	}
	cond := meta.FindStatusCondition(sb.Status.Conditions, kubeparkv1alpha1.ConditionRBACReady)
	if cond == nil {
		return ""
	}
	return string(cond.Status) + "/" + cond.Reason
}

var _ = Describe("AccessProfile RBAC", func() {
	Context("a permitted profile", func() {
		It("creates the SA, shared Role and RoleBinding, and mounts the SA on the pod", func() {
			target := fmt.Sprintf("ap-target-%d", profileCounter+1)
			createNamespace(target)
			createProfile("ap-ok", metav1.NamespaceDefault, target)
			createTemplate("tpl-ap-ok")

			sb := newSandbox("tpl-ap-ok")
			sb.Spec.AccessProfile = "ap-ok"
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())

			By("creating the per-sandbox ServiceAccount")
			Eventually(func() error {
				var sa corev1.ServiceAccount
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: saName(sb.Name)}, &sa)
			}, 15*time.Second, 300*time.Millisecond).Should(Succeed())

			By("creating the shared profile Role in the grant namespace")
			Eventually(func() error {
				var role rbacv1.Role
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: target, Name: profileRoleName("ap-ok")}, &role)
			}, 15*time.Second, 300*time.Millisecond).Should(Succeed())

			By("creating the sandbox RoleBinding in the grant namespace")
			Eventually(func() error {
				var rb rbacv1.RoleBinding
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: target, Name: roleBindingName(sb.Name)}, &rb)
			}, 15*time.Second, 300*time.Millisecond).Should(Succeed())

			By("mounting the SA on the pod")
			Eventually(func() string {
				var pod corev1.Pod
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sb.Namespace, Name: podspec.PodName(sb.Name)}, &pod); err != nil {
					return ""
				}
				return pod.Spec.ServiceAccountName
			}, 15*time.Second, 300*time.Millisecond).Should(Equal(saName(sb.Name)))

			By("reporting RBACReady=True")
			Eventually(func() string { return rbacReadyReason(sb.Name) },
				10*time.Second, 300*time.Millisecond).Should(Equal("True/Running"))
		})
	})

	Context("a profile that does not permit the sandbox namespace", func() {
		It("refuses with ProfileNotPermitted and creates no SA", func() {
			target := fmt.Sprintf("ap-target-%d", profileCounter+1)
			createNamespace(target)
			createProfile("ap-denied", "some-other-ns", target)
			createTemplate("tpl-ap-denied")

			sb := newSandbox("tpl-ap-denied")
			sb.Spec.AccessProfile = "ap-denied"
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())

			Eventually(func() string { return rbacReadyReason(sb.Name) },
				15*time.Second, 300*time.Millisecond).Should(Equal("False/ProfileNotPermitted"))

			var sa corev1.ServiceAccount
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: saName(sb.Name)}, &sa)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("deleting a referenced profile", func() {
		It("GCs its Roles and flips referencing sandboxes to ProfileDeleted", func() {
			target := fmt.Sprintf("ap-target-%d", profileCounter+1)
			createNamespace(target)
			createProfile("ap-del", metav1.NamespaceDefault, target)
			createTemplate("tpl-ap-del")

			sb := newSandbox("tpl-ap-del")
			sb.Spec.AccessProfile = "ap-del"
			Expect(k8sClient.Create(ctx, sb)).To(Succeed())
			Eventually(func() string { return rbacReadyReason(sb.Name) },
				15*time.Second, 300*time.Millisecond).Should(Equal("True/Running"))

			By("deleting the profile")
			var profile kubeparkv1alpha1.AccessProfile
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ap-del"}, &profile)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &profile)).To(Succeed())

			By("removing the shared Role")
			Eventually(func() bool {
				var role rbacv1.Role
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: target, Name: profileRoleName("ap-del")}, &role)
				return apierrors.IsNotFound(err)
			}, 15*time.Second, 300*time.Millisecond).Should(BeTrue())

			By("flipping the sandbox to ProfileDeleted")
			Eventually(func() string { return rbacReadyReason(sb.Name) },
				15*time.Second, 300*time.Millisecond).Should(Equal("False/ProfileDeleted"))
		})
	})

	Context("a grant namespace that does not exist", func() {
		It("applies existing grants and reports Valid=False/MissingNamespace", func() {
			createNamespace("ap-present")
			createProfile("ap-partial", metav1.NamespaceDefault, "ap-present", "ap-absent")

			Eventually(func() error {
				var role rbacv1.Role
				return k8sClient.Get(ctx, types.NamespacedName{Namespace: "ap-present", Name: profileRoleName("ap-partial")}, &role)
			}, 15*time.Second, 300*time.Millisecond).Should(Succeed())

			Eventually(func() string {
				var p kubeparkv1alpha1.AccessProfile
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "ap-partial"}, &p); err != nil {
					return ""
				}
				cond := meta.FindStatusCondition(p.Status.Conditions, kubeparkv1alpha1.ConditionValid)
				if cond == nil {
					return ""
				}
				return string(cond.Status) + "/" + cond.Reason
			}, 15*time.Second, 300*time.Millisecond).Should(Equal("False/MissingNamespace"))
		})
	})
})
