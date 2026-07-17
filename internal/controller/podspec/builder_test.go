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

package podspec

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

const (
	testImage   = "img"
	volNameHome = "home"
)

func testSandbox() *kubeparkv1alpha1.Sandbox {
	return &kubeparkv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "alice", UID: types.UID("uid-123")},
		Spec: kubeparkv1alpha1.SandboxSpec{
			Template: "ops",
			Owner:    kubeparkv1alpha1.OwnerSpec{Name: "alice@example.com"},
		},
	}
}

func testTemplate() *kubeparkv1alpha1.SandboxTemplate {
	return &kubeparkv1alpha1.SandboxTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "ops"},
		Spec: kubeparkv1alpha1.SandboxTemplateSpec{
			Image:    "ghcr.io/example/ops:latest",
			HomeSize: resource.MustParse("5Gi"),
		},
	}
}

func TestBuildPod_SecurityBaseline(t *testing.T) {
	pod := BuildPod(testSandbox(), testTemplate(), Options{AgentImage: "ghcr.io/frauniki/kubepark:test"})

	sc := pod.Spec.SecurityContext
	if sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Error("expected runAsNonRoot=true")
	}
	if sc.RunAsUser == nil || *sc.RunAsUser != 1000 {
		t.Errorf("expected default runAsUser 1000, got %v", sc.RunAsUser)
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Error("expected seccomp RuntimeDefault")
	}
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		t.Error("expected automountServiceAccountToken=false when the sandbox has no AccessProfile SA")
	}
	if got := pod.Annotations["cluster-autoscaler.kubernetes.io/safe-to-evict"]; got != "false" {
		t.Errorf("expected safe-to-evict=false, got %q", got)
	}
}

// The user CA private key must never reach a sandbox pod: only the agent
// binary (via emptyDir) and the home volume are mounted.
func TestBuildPod_NoSecretVolumes(t *testing.T) {
	pod := BuildPod(testSandbox(), testTemplate(), Options{AgentImage: testImage})
	for _, v := range pod.Spec.Volumes {
		if v.Secret != nil {
			t.Errorf("sandbox pod must not mount any Secret volume, found %q", v.Name)
		}
	}
}

func TestBuildPod_AgentWrapsCommand(t *testing.T) {
	tpl := testTemplate()
	tpl.Spec.Command = []string{"sleep", "infinity"}
	pod := BuildPod(testSandbox(), tpl, Options{AgentImage: testImage})

	c := pod.Spec.Containers[0]
	if len(c.Command) == 0 || c.Command[0] != agentDir+"/agent" {
		t.Errorf("expected agent as entrypoint, got %v", c.Command)
	}
	if len(c.Args) < 1 || c.Args[0] != "--" {
		t.Errorf("expected template command passed after --, got %v", c.Args)
	}
	if pod.Spec.InitContainers[0].Args[0] != "agent" || pod.Spec.InitContainers[0].Args[1] != "install" {
		t.Errorf("expected agent install init container, got %v", pod.Spec.InitContainers[0].Args)
	}
}

func TestBuildPod_ServiceAccountMountsToken(t *testing.T) {
	pod := BuildPod(testSandbox(), testTemplate(), Options{AgentImage: testImage, ServiceAccountName: "kubepark-sb-demo"})
	if pod.Spec.ServiceAccountName != "kubepark-sb-demo" {
		t.Errorf("expected SA set, got %q", pod.Spec.ServiceAccountName)
	}
	if pod.Spec.AutomountServiceAccountToken == nil || !*pod.Spec.AutomountServiceAccountToken {
		t.Error("expected automountServiceAccountToken=true when an SA is set")
	}
}

func TestBuildPod_EmptyCommand(t *testing.T) {
	pod := BuildPod(testSandbox(), testTemplate(), Options{AgentImage: testImage})
	if got := pod.Spec.Containers[0].Args; len(got) != 0 {
		t.Errorf("expected no args when template command is empty, got %v", got)
	}
}

func TestBuildPod_StrongIsolationRuntimeClass(t *testing.T) {
	tpl := testTemplate()
	tpl.Spec.IsolationLevel = kubeparkv1alpha1.IsolationStrong
	tpl.Spec.RuntimeClassName = ptr.To("gvisor")
	pod := BuildPod(testSandbox(), tpl, Options{AgentImage: testImage})
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "gvisor" {
		t.Errorf("expected runtimeClassName gvisor, got %v", pod.Spec.RuntimeClassName)
	}
}

func TestBuildPod_ExistingClaim(t *testing.T) {
	sb := testSandbox()
	sb.Spec.Home = &kubeparkv1alpha1.HomeSpec{ExistingClaim: "shared-home"}
	pod := BuildPod(sb, testTemplate(), Options{AgentImage: testImage})
	for _, v := range pod.Spec.Volumes {
		if v.Name == volNameHome {
			if v.PersistentVolumeClaim.ClaimName != "shared-home" {
				t.Errorf("expected existing claim, got %q", v.PersistentVolumeClaim.ClaimName)
			}
			return
		}
	}
	t.Fatal("home volume not found")
}

func TestBuildPVC_SizeOverride(t *testing.T) {
	sb := testSandbox()
	size := resource.MustParse("20Gi")
	sb.Spec.Home = &kubeparkv1alpha1.HomeSpec{Size: &size}
	pvc := BuildPVC(sb, testTemplate())
	if got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(size) != 0 {
		t.Errorf("expected 20Gi override, got %v", got.String())
	}
	if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expected RWO, got %v", pvc.Spec.AccessModes)
	}
}

func TestTemplateHash_StableAndSensitive(t *testing.T) {
	tpl := testTemplate()
	h1 := TemplateHash(&tpl.Spec)
	if h1 != TemplateHash(&tpl.Spec) {
		t.Error("hash must be stable for identical specs")
	}
	tpl.Spec.Image = "ghcr.io/example/ops:v2"
	if h1 == TemplateHash(&tpl.Spec) {
		t.Error("hash must change when the spec changes")
	}
}
