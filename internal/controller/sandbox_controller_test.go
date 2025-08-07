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
	"k8s.io/apimachinery/pkg/api/resource"
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

		Context("With SSH username configuration", func() {
			It("should set SSH_USERNAME environment variable when username is specified", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with custom SSH username")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  "testuser",
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

				By("Checking if Pod was created with SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check if SSH_USERNAME environment variable is set
				container := pod.Spec.Containers[0]
				var sshUsernameEnv *corev1.EnvVar
				for _, env := range container.Env {
					if env.Name == SSHUsernameEnvVar {
						sshUsernameEnv = &env
						break
					}
				}
				Expect(sshUsernameEnv).NotTo(BeNil())
				Expect(sshUsernameEnv.Value).To(Equal("testuser"))
			})

			It("should not set SSH_USERNAME environment variable when username is not specified", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-no-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource without SSH username")
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

				By("Checking if Pod was created without SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check that SSH_USERNAME environment variable is not set
				container := pod.Spec.Containers[0]
				for _, env := range container.Env {
					Expect(env.Name).NotTo(Equal(SSHUsernameEnvVar))
				}
			})

			It("should handle empty username gracefully", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-empty-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with empty SSH username")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  "", // Empty username
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

				By("Checking if Pod was created without SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check that SSH_USERNAME environment variable is not set for empty username
				container := pod.Spec.Containers[0]
				for _, env := range container.Env {
					Expect(env.Name).NotTo(Equal(SSHUsernameEnvVar))
				}
			})

			It("should handle special characters in username", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-special-chars"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with special characters in username")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  "test-user_123", // Username with special characters
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

				By("Checking if Pod was created with SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check if SSH_USERNAME environment variable is set correctly
				container := pod.Spec.Containers[0]
				var sshUsernameEnv *corev1.EnvVar
				for _, env := range container.Env {
					if env.Name == SSHUsernameEnvVar {
						sshUsernameEnv = &env
						break
					}
				}
				Expect(sshUsernameEnv).NotTo(BeNil())
				Expect(sshUsernameEnv.Value).To(Equal("test-user_123"))
			})

			It("should handle long username", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-long-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				longUsername := "verylongusernamethatexceedsnormallimits123456789"
				By("Creating a Sandbox resource with long username")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  longUsername,
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

				By("Checking if Pod was created with SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check if SSH_USERNAME environment variable is set correctly
				container := pod.Spec.Containers[0]
				var sshUsernameEnv *corev1.EnvVar
				for _, env := range container.Env {
					if env.Name == SSHUsernameEnvVar {
						sshUsernameEnv = &env
						break
					}
				}
				Expect(sshUsernameEnv).NotTo(BeNil())
				Expect(sshUsernameEnv.Value).To(Equal(longUsername))
			})

			It("should handle username with whitespace", func() {
				// Use test-specific name
				sandboxName := "test-sandbox-whitespace-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with whitespace in username")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  " testuser ", // Username with leading/trailing whitespace
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

				By("Checking if Pod was created with SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check if SSH_USERNAME environment variable is set with the exact value (including whitespace)
				container := pod.Spec.Containers[0]
				var sshUsernameEnv *corev1.EnvVar
				for _, env := range container.Env {
					if env.Name == SSHUsernameEnvVar {
						sshUsernameEnv = &env
						break
					}
				}
				Expect(sshUsernameEnv).NotTo(BeNil())
				Expect(sshUsernameEnv.Value).To(Equal(" testuser "))
			})
		})

		Context("With SandboxTemplate and SSH username", func() {
			It("should prioritize Sandbox username over SandboxTemplate username", func() {
				// Use test-specific name
				templateName := "test-template-username"
				sandboxName := "test-sandbox-template-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a SandboxTemplate with SSH username")
				template := &kubeparkv1alpha1.SandboxTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      templateName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:template",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  "templateuser",
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 template@example.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, template)).To(Succeed())
				defer func() {
					By("Cleaning up the SandboxTemplate resource")
					Expect(k8sClient.Delete(ctx, template)).To(Succeed())
				}()

				By("Creating a Sandbox resource that references the template but overrides username")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						SandboxTemplateRef: &kubeparkv1alpha1.SandboxTemplateRef{
							Name: templateName,
						},
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  "overrideuser", // This should take precedence
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 override@example.com",
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

				By("Checking if Pod was created with overridden SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check if SSH_USERNAME environment variable uses the Sandbox value, not template value
				container := pod.Spec.Containers[0]
				var sshUsernameEnv *corev1.EnvVar
				for _, env := range container.Env {
					if env.Name == SSHUsernameEnvVar {
						sshUsernameEnv = &env
						break
					}
				}
				Expect(sshUsernameEnv).NotTo(BeNil())
				Expect(sshUsernameEnv.Value).To(Equal("overrideuser"))
			})

			It("should inherit username from SandboxTemplate when not specified in Sandbox", func() {
				// Use test-specific name
				templateName := "test-template-inherit-username"
				sandboxName := "test-sandbox-inherit-username"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a SandboxTemplate with SSH username")
				template := &kubeparkv1alpha1.SandboxTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      templateName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:template",
						SSH: &kubeparkv1alpha1.SSHConfig{
							Username:  "inheriteduser",
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 template@example.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, template)).To(Succeed())
				defer func() {
					By("Cleaning up the SandboxTemplate resource")
					Expect(k8sClient.Delete(ctx, template)).To(Succeed())
				}()

				By("Creating a Sandbox resource that references the template without SSH config")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						SandboxTemplateRef: &kubeparkv1alpha1.SandboxTemplateRef{
							Name: templateName,
						},
						// No SSH config specified - should inherit from template
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

				By("Checking if Pod was created with inherited SSH_USERNAME environment variable")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				// Check if SSH_USERNAME environment variable uses the inherited value from template
				container := pod.Spec.Containers[0]
				var sshUsernameEnv *corev1.EnvVar
				for _, env := range container.Env {
					if env.Name == SSHUsernameEnvVar {
						sshUsernameEnv = &env
						break
					}
				}
				Expect(sshUsernameEnv).NotTo(BeNil())
				Expect(sshUsernameEnv.Value).To(Equal("inheriteduser"))
			})
		})

		Context("With Container configuration", func() {
			It("should apply container resource limits and requests", func() {
				sandboxName := "test-sandbox-resources"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with container resources")
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
						Container: &corev1.Container{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
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

				By("Checking if Pod was created with resource limits and requests")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				container := pod.Spec.Containers[0]
				Expect(container.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
				Expect(container.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("512Mi")))
				Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
				Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("128Mi")))
			})

			It("should apply container environment variables", func() {
				sandboxName := "test-sandbox-env"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with container environment variables")
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
						Container: &corev1.Container{
							Env: []corev1.EnvVar{
								{Name: "TEST_ENV", Value: "test_value"},
								{Name: "ANOTHER_ENV", Value: "another_value"},
							},
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

				By("Checking if Pod was created with environment variables")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers).To(HaveLen(1))

				container := pod.Spec.Containers[0]
				envMap := make(map[string]string)
				for _, env := range container.Env {
					envMap[env.Name] = env.Value
				}
				Expect(envMap).To(HaveKeyWithValue("TEST_ENV", "test_value"))
				Expect(envMap).To(HaveKeyWithValue("ANOTHER_ENV", "another_value"))
			})
		})

		Context("With HostNetwork configuration", func() {
			It("should set HostNetwork when specified", func() {
				sandboxName := "test-sandbox-hostnetwork"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				hostNetwork := true
				By("Creating a Sandbox resource with HostNetwork enabled")
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
						HostNetwork: &hostNetwork,
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

				By("Checking if Pod was created with HostNetwork enabled")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.HostNetwork).To(BeTrue())
			})
		})

		Context("With ServiceAccount configuration", func() {
			It("should use custom ServiceAccount when specified", func() {
				sandboxName := "test-sandbox-serviceaccount"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with custom ServiceAccount")
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
						ServiceAccountName: "custom-service-account",
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

				By("Checking if Pod was created with custom ServiceAccount")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.ServiceAccountName).To(Equal("custom-service-account"))
			})

			It("should use default ServiceAccount when not specified", func() {
				sandboxName := "test-sandbox-default-serviceaccount"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource without ServiceAccount")
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

				By("Checking if Pod was created with default ServiceAccount")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.ServiceAccountName).To(Equal("default"))
			})
		})

		Context("With ImagePullPolicy defaults", func() {
			It("should use PullAlways for :latest tag", func() {
				sandboxName := "test-sandbox-pullalways"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with :latest image")
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

				By("Checking if Pod was created with PullAlways policy")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
			})

			It("should use PullIfNotPresent for versioned tag", func() {
				sandboxName := "test-sandbox-pullif"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with versioned image")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:v1.0.0",
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

				By("Checking if Pod was created with PullIfNotPresent policy")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			})
		})

		Context("With TerminationGracePeriodSeconds", func() {
			It("should use custom termination grace period", func() {
				sandboxName := "test-sandbox-grace-period"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				gracePeriod := int64(120)
				By("Creating a Sandbox resource with custom termination grace period")
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
						TerminationGracePeriodSeconds: &gracePeriod,
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

				By("Checking if Pod was created with custom termination grace period")
				podName := fmt.Sprintf("sandbox-%s", sandboxName)
				pod := &corev1.Pod{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: sandboxNamespace,
				}, pod)
				Expect(err).NotTo(HaveOccurred())
				Expect(*pod.Spec.TerminationGracePeriodSeconds).To(Equal(gracePeriod))
			})
		})

		Context("With ConfigMap updates", func() {
			It("should update ConfigMap when SSH public key changes", func() {
				sandboxName := "test-sandbox-configmap-update"
				typeNamespacedName := types.NamespacedName{
					Name:      sandboxName,
					Namespace: sandboxNamespace,
				}

				By("Creating a Sandbox resource with initial SSH public key")
				sandbox := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sandboxName,
						Namespace: sandboxNamespace,
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 initial@example.com",
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

				By("Checking initial ConfigMap")
				configMapName := fmt.Sprintf("ssh-public-key-%s", sandboxName)
				configMap := &corev1.ConfigMap{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      configMapName,
					Namespace: sandboxNamespace,
				}, configMap)
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Data["authorized_keys"]).To(ContainSubstring("initial@example.com"))

				By("Updating the SSH public key")
				err = k8sClient.Get(ctx, typeNamespacedName, sandbox)
				Expect(err).NotTo(HaveOccurred())
				sandbox.Spec.SSH.PublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOLGGiT2RiSisxJxb+Y5yI2ifFgYZlD1TdH5SSl9Iqk9 updated@example.com"
				err = k8sClient.Update(ctx, sandbox)
				Expect(err).NotTo(HaveOccurred())

				By("Reconciling again")
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Checking updated ConfigMap")
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      configMapName,
					Namespace: sandboxNamespace,
				}, configMap)
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Data["authorized_keys"]).To(ContainSubstring("updated@example.com"))
			})
		})
	})
})
