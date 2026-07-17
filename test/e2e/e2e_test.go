//go:build e2e

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

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/frauniki/kubepark/test/utils"
)

// This suite drives the full kubepark loop against a Kind cluster: apply a
// template and a sandbox, wait for the pod, sign a certificate offline, and
// SSH in through the gateway to run a command.
//
// NetworkPolicy enforcement and OIDC login are out of scope here (they need
// a NetworkPolicy-enforcing CNI and a real IdP respectively); those are
// documented and verified separately.

const sandboxManifest = `
apiVersion: kubepark.dev/v1alpha1
kind: SandboxTemplate
metadata: {name: e2e-shell}
spec:
  image: alpine:3.20
  command: ["sleep", "infinity"]
  homeSize: 1Gi
---
apiVersion: kubepark.dev/v1alpha1
kind: Sandbox
metadata: {name: e2e-demo, namespace: default}
spec:
  template: e2e-shell
  owner: {name: e2e@example.com}
`

var _ = Describe("kubepark end to end", Ordered, func() {
	var cli string

	BeforeAll(func() {
		By("building the kubepark CLI")
		dir := GinkgoT().TempDir()
		cli = filepath.Join(dir, "kubepark")
		_, err := utils.Run(command("go", "build", "-o", cli, "./cmd/kubepark"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("provisions a sandbox and reaches an SSH shell through the gateway", func() {
		By("applying the template and sandbox")
		Expect(applyStdin(sandboxManifest)).To(Succeed())
		DeferCleanup(func() {
			_, _ = utils.Run(command("kubectl", "delete", "sandbox", "e2e-demo",
				"-n", "default", "--ignore-not-found"))
			_, _ = utils.Run(command("kubectl", "delete", "sandboxtemplate", "e2e-shell",
				"--ignore-not-found"))
		})

		By("waiting for the sandbox pod to be ready")
		Eventually(func() error {
			_, err := utils.Run(command("kubectl", "wait", "--for=condition=ready",
				"pod", "-l", "kubepark.dev/sandbox=e2e-demo", "-n", "default", "--timeout=10s"))
			return err
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("signing a certificate offline")
		// Isolate ~/.kubepark by overriding HOME, but keep kubeconfig
		// reachable by pinning KUBECONFIG to its real path first.
		if os.Getenv("KUBECONFIG") == "" {
			os.Setenv("KUBECONFIG", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		}
		home := GinkgoT().TempDir()
		os.Setenv("HOME", home)
		_, err := utils.Run(command(cli, "admin", "sign-cert",
			"--principal", "e2e@example.com", "--ca-namespace", namespace))
		Expect(err).NotTo(HaveOccurred())

		By("extracting the host CA public key")
		hostCA := filepath.Join(home, "host-ca.pub")
		out, err := utils.Run(command("kubectl", "-n", namespace, "get", "secret", "kubepark-ca",
			"-o", "jsonpath={.data.host-ca\\.pub}"))
		Expect(err).NotTo(HaveOccurred())
		Expect(writeBase64(hostCA, out)).To(Succeed())

		By("port-forwarding the gateway")
		pf := command("kubectl", "-n", namespace, "port-forward",
			"svc/kubepark-gateway", "12222:2222")
		Expect(pf.Start()).To(Succeed())
		DeferCleanup(func() { _ = pf.Process.Kill() })
		time.Sleep(4 * time.Second)

		By("generating the ssh_config")
		_, err = utils.Run(command(cli, "ssh", "e2e-demo", "-n", "default",
			"--gateway", "localhost:12222", "--host-ca", hostCA, "--print-config"))
		Expect(err).NotTo(HaveOccurred())

		By("running a command over SSH through the gateway")
		// Verify identity with `id -u` rather than `whoami`: the sandbox runs
		// as uid 1000 with no /etc/passwd entry, so `whoami` exits non-zero
		// ("unknown uid 1000") even though the shell is fully functional.
		sshCfg := filepath.Join(home, ".kubepark", "ssh_config")
		Eventually(func() (string, error) {
			return utils.Run(command("ssh", "-F", sshCfg, "-o", "BatchMode=yes",
				"-o", "ConnectTimeout=30", "e2e-demo.default", "echo e2e-ok; id -u"))
		}, time.Minute, 5*time.Second).Should(ContainSubstring("e2e-ok"))
	})
})

// applyStdin runs `kubectl apply -f -` with the given manifest.
func applyStdin(manifest string) error {
	cmd := command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := utils.Run(cmd)
	return err
}

// writeBase64 decodes a base64 string into a file.
func writeBase64(path, b64 string) error {
	cmd := exec.Command("base64", "-d")
	cmd.Stdin = strings.NewReader(b64)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	cmd.Stdout = f
	return cmd.Run()
}
