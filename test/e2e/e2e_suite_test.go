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
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/frauniki/kubepark/test/utils"
)

// image is the kubepark image built and loaded into Kind for the suite. It
// can be overridden with E2E_IMG.
var image = envOr("E2E_IMG", "example.com/kubepark:e2e")

const (
	namespace   = "kubepark-system"
	releaseName = "kubepark"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestE2E runs the kubepark e2e suite against a pre-existing Kind cluster.
// It builds and loads the image, helm-installs the chart, and drives a
// sandbox from creation to an SSH shell.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting kubepark e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the kubepark image")
	_, err := utils.Run(command("make", "docker-build", "IMG="+image))
	Expect(err).NotTo(HaveOccurred(), "failed to build the image")

	By("loading the image into Kind")
	Expect(loadImage(image)).To(Succeed(), "failed to load the image into Kind")

	By("installing the chart via Helm")
	_, err = utils.Run(command("helm", "upgrade", "--install", releaseName,
		"charts/kubepark",
		"--namespace", namespace, "--create-namespace",
		"--set", "image.repository="+imageRepo(image),
		"--set", "image.tag="+imageTag(image),
		"--set", "image.pullPolicy=IfNotPresent",
		"--set-string", "crds.keep=false",
		"--set", "gateway.service.type=ClusterIP",
		"--wait", "--timeout", "180s",
	))
	Expect(err).NotTo(HaveOccurred(), "failed to helm install kubepark")

	By("pointing the operator at the loaded agent image")
	_, err = utils.Run(command("kubectl", "-n", namespace, "set", "env",
		"deploy/"+releaseName, "POD_NAMESPACE="+namespace))
	Expect(err).NotTo(HaveOccurred())
	_, err = utils.Run(command("kubectl", "-n", namespace, "patch", "deploy", releaseName,
		"--type=json", "-p",
		`[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--agent-image=`+image+`"}]`))
	Expect(err).NotTo(HaveOccurred())
	_, err = utils.Run(command("kubectl", "-n", namespace, "rollout", "status",
		"deploy/"+releaseName, "--timeout=120s"))
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("uninstalling the chart")
	_, _ = utils.Run(command("helm", "uninstall", releaseName, "--namespace", namespace))
})
