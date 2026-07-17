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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/frauniki/kubepark/internal/gateway"
)

// newAdminCommand groups administrative helpers that hold direct CA access.
// They are a stopgap for issuing certificates before OIDC login (M4) and
// the backbone of the e2e tests.
func newAdminCommand() *cobra.Command {
	admin := &cobra.Command{
		Use:   "admin",
		Short: "Administrative helpers (direct CA access)",
	}
	admin.AddCommand(newSignCertCommand())
	return admin
}

func newSignCertCommand() *cobra.Command {
	var principal string
	var namespace string
	var ttl time.Duration
	cmd := &cobra.Command{
		Use:   "sign-cert",
		Short: "Sign the local CLI public key with the user CA Secret (needs kubeconfig)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if principal == "" {
				return fmt.Errorf("--principal is required")
			}
			return runSignCert(cmd.Context(), principal, namespace, ttl)
		},
	}
	cmd.Flags().StringVar(&principal, "principal", "", "Certificate principal (the sandbox owner identity).")
	cmd.Flags().StringVar(&namespace, "ca-namespace", "kubepark-system", "Namespace holding the kubepark-ca Secret.")
	cmd.Flags().DurationVar(&ttl, "ttl", 8*time.Hour, "Certificate validity.")
	return cmd
}

func runSignCert(ctx context.Context, principal, namespace string, ttl time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, pubPath, err := ensureKeyPair()
	if err != nil {
		return err
	}
	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return err
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return err
	}
	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "kubepark-ca"}, &secret); err != nil {
		return fmt.Errorf("read kubepark-ca secret: %w", err)
	}

	signer, err := gateway.NewSigner(secret.Data["user-ca.key"], ttl)
	if err != nil {
		return err
	}
	certBytes, err := signer.Sign(pub, principal)
	if err != nil {
		return err
	}

	outPath, err := certPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, certBytes, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote certificate for %q (valid %s) to %s\n", principal, ttl, outPath)
	return nil
}
