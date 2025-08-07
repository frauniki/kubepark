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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

var _ = Describe("Sandbox Controller", func() {
	Context("When reconciling a Sandbox resource", func() {
		const sandboxNamespace = "default"

		ctx := context.Background()

		Context("With valid SSH configuration", func() {
			It("should create a ConfigMap with SSH public key", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-configmap"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with SSH config")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 test@example.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, sandbox)).To(Succeed())
				defer func() {
					By("Cleaning up the Sandbox resource")
					Expect(k8sClient.Delete(ctx, sandbox)).To(Succeed())
				}()
				By("Reconciling the Sandbox")
				controllerReconciler := &SandboxReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Checking if ConfigMap was created")
				configMapName := fmt.Sprintf("ssh-public-key-%s", sandboxName)
				configMap := &corev1.ConfigMap{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      configMapName,
					Namespace: sandboxNamespace,
				}, configMap)
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Data["authorized_keys"]).To(ContainSubstring("ssh-ed25519"))
			})

			It("should create a Pod for the Sandbox", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-pod"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with SSH config")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 test@example.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, sandbox)).To(Succeed())
				defer func() {
					By("Cleaning up the Sandbox resource")
					Expect(k8sClient.Delete(ctx, sandbox)).To(Succeed())
				}()
				By("Reconciling the Sandbox")
				controllerReconciler := &SandboxReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Checking if Pod was created")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))
				Expect(pod.Spec.Containers[0].Image).To(Equal("kubepark/sandbox-ssh:latest"))
			})

			It("should update Sandbox status", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-status"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with SSH config")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 test@example.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, sandbox)).To(Succeed())
				defer func() {
					By("Cleaning up the Sandbox resource")
					Expect(k8sClient.Delete(ctx, sandbox)).To(Succeed())
				}()
				By("Reconciling the Sandbox")
				controllerReconciler := &SandboxReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Checking Sandbox status")
				updatedSandbox := &kubeparkv1alpha1.Sandbox{}
				err = k8sClient.Get(ctx, typeNamespacedName, updatedSandbox)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedSandbox.Status.Phase).To(Equal(kubeparkv1alpha1.SandboxPending))
			})
		})

		Context("Without SSH configuration", func() {
			It("should fail reconciliation when SSH public key is missing", func() {
				By("Creating a Sandbox without SSH config")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-ssh-sandbox",
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
					},
				}
				Expect(k8sClient.Create(ctx, sandbox)).To(Succeed())

				By("Reconciling the Sandbox")
				controllerReconciler := &SandboxReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "no-ssh-sandbox",
						Namespace: sandboxNamespace,
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("SSH public key is required"))

				By("Cleaning up")
				Expect(k8sClient.Delete(ctx, sandbox)).To(Succeed())
			})
		})

		Context("With custom ImagePullPolicy", func() {
			It("should use specified ImagePullPolicy", func() {
				By("Creating a Sandbox with custom ImagePullPolicy")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom-pull-policy",
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image:           "kubepark/sandbox-ssh:v1.0.0",
						ImagePullPolicy: corev1.PullIfNotPresent,
						SSH: &kubeparkv1alpha1.SSHConfig{
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9",
						},
					},
				}
				Expect(k8sClient.Create(ctx, sandbox)).To(Succeed())

				By("Reconciling the Sandbox")
				controllerReconciler := &SandboxReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "custom-pull-policy",
						Namespace: sandboxNamespace,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Checking Pod ImagePullPolicy")
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      "sandbox-custom-pull-policy",
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

				By("Cleaning up")
				Expect(k8sClient.Delete(ctx, sandbox)).To(Succeed())
			})
		})
	})
})
