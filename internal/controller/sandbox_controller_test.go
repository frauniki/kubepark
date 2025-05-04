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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

var _ = Describe("Sandbox Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		sandbox := &kubeparkv1alpha1.Sandbox{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Sandbox")
			err := k8sClient.Get(ctx, typeNamespacedName, sandbox)
			if err != nil && errors.IsNotFound(err) {
				resource := &kubeparkv1alpha1.Sandbox{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kubeparkv1alpha1.SandboxSpec{
						Image: "kubepark/sandbox-ssh:latest",
						SSH: &kubeparkv1alpha1.SSHConfig{
							PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDTd4/QQEZiVlP8YdOiGrF8sFi+n7G+pPX7JFaXxeP5v/xZoLp1CIYTUfgfs8FYM4VjU8PvcQtM5r0DNwOcLkx7l7XEKyICxR2bV5QGUmFe2cZ7Vvh9ooEl/LFdogxgvRT9qUYZnrY4BtEuW0O7P3NQQ9IfEh+wPg+31xJj4JCXQWZCJxiGgEGGUTyuZ8jpcuJ5zKWdIWmQrZMIgpzjLiXXbEh8xNYrE0DO5mLKKxuQL2ik4KfAZDQC9ZPC0T+Z8L9U9pGSEaopkPQA9C9U0hT9L6mFoEzQZN5GxkKIESz2BOAlQpSevp6jTLVr3FGIGrxwzNQ1Vy8NJJfTnNrBW4HyEQZXJkGxKxMGIZVGzCXQ1VeX9UvvJi3UrOKnJ/sMF6/nKqxjBzHWbwxTr+n8l2EGJQ2ayLnLlEtVKRGBKz+G7i3woEQWWNxN1xhOQUL3ZYveGnmcYA0WcQqzwQzQxoKImnUJt+84+LJ4QZKq/XKS7BBCQQa5/5jBJQtQdYk= user@example.com",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &kubeparkv1alpha1.Sandbox{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Sandbox")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SandboxReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
