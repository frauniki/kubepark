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

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/controller"
	"github.com/frauniki/kubepark/internal/gateway"
	"github.com/frauniki/kubepark/internal/sshca"
)

// newGatewayCommand runs the kubepark gateway: an SSH jump host that
// authenticates clients with CA-signed certificates and routes direct-tcpip
// channels to sandbox pods. (The HTTP reverse proxy lands in M5.)
func newGatewayCommand() *cobra.Command {
	var addr string
	var defaultNamespace string
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Run the kubepark SSH/HTTP gateway",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGateway(cmd.Context(), addr, defaultNamespace)
		},
	}
	cmd.Flags().StringVar(&addr, "ssh-address", ":2222", "SSH jump host listen address.")
	cmd.Flags().StringVar(&defaultNamespace, "default-namespace", "", "Namespace assumed when a target omits one.")
	return cmd
}

func runGateway(ctx context.Context, addr, defaultNamespace string) error {
	if ctx == nil {
		ctx = ctrl.SetupSignalHandler()
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubeparkv1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		return fmt.Errorf("build manager: %w", err)
	}

	// Load CA + gateway host key up front (fail fast on misconfiguration).
	direct, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	ns := controller.OperatorNamespace()
	caSecret, err := controller.EnsureCASecret(ctx, direct, ns)
	if err != nil {
		return fmt.Errorf("load CA secret: %w", err)
	}
	hostKey, err := gatewayHostKey(ctx, direct, ns)
	if err != nil {
		return err
	}

	server, err := gateway.NewSSHServer(gateway.SSHConfig{
		Addr:             addr,
		HostKeyPEM:       hostKey,
		UserCAAuthorized: caSecret.Data[controller.KeyUserCAPublic],
		DefaultNamespace: defaultNamespace,
		Store:            gateway.NewStore(mgr.GetClient()),
		Dialer:           gateway.NewDirectDialer(),
	})
	if err != nil {
		return err
	}

	// Run the SSH server alongside the manager (for its cached client).
	if err := mgr.Add(&sshRunnable{server: server}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "kubepark gateway SSH jump host listening on %s\n", addr)
	return mgr.Start(ctx)
}

// gatewayHostKey loads the gateway's own SSH host key Secret, generating it
// on first use so a fresh install needs no manual bootstrap.
func gatewayHostKey(ctx context.Context, c client.Client, namespace string) ([]byte, error) {
	const name = "kubepark-gateway-hostkey"
	var secret corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &secret)
	if err == nil {
		return secret.Data["ssh_host_ed25519_key"], nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	kp, err := sshca.GenerateKeyPair("kubepark-gateway")
	if err != nil {
		return nil, err
	}
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string][]byte{
			"ssh_host_ed25519_key":     kp.PrivatePEM,
			"ssh_host_ed25519_key.pub": kp.PublicAuthorized,
		},
	}
	if err := c.Create(ctx, &secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			var existing corev1.Secret
			if getErr := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &existing); getErr == nil {
				return existing.Data["ssh_host_ed25519_key"], nil
			}
		}
		return nil, err
	}
	return kp.PrivatePEM, nil
}

// sshRunnable adapts the SSH server to a manager Runnable so it shares the
// manager lifecycle.
type sshRunnable struct {
	server interface {
		ListenAndServe() error
		Close() error
	}
}

func (s *sshRunnable) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.server.Close()
	}()
	if err := s.server.ListenAndServe(); err != nil && err.Error() != "ssh: Server closed" {
		return err
	}
	return nil
}
