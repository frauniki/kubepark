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

// Package podspec builds the executor pod for a sandbox as a pure function
// of the Sandbox, its SandboxTemplate and operator-level options, so the
// pod shape is unit-testable without a cluster.
package podspec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

const (
	// LabelSandbox holds the sandbox name on all owned resources.
	LabelSandbox = "kubepark.dev/sandbox"
	// LabelSandboxUID holds the sandbox UID for cross-namespace GC.
	LabelSandboxUID = "kubepark.dev/sandbox-uid"
	// LabelComponent marks kubepark-managed pods ("sandbox", "gateway").
	LabelComponent = "kubepark.dev/component"
	// LabelManagedBy is the standard managed-by label value.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// ComponentSandbox is the LabelComponent value for sandbox pods.
	ComponentSandbox = "sandbox"

	// AgentPort is the fixed in-pod SSH port served by the agent.
	AgentPort = 2222

	// HomeMountPath is where the home PVC is mounted. The image ENTRYPOINT
	// is not used and /home/sandbox is the documented home directory.
	HomeMountPath = "/home/sandbox"

	// agentDir is the emptyDir shared between the init container (which
	// self-copies the agent binary) and the user container. It must not
	// collide with the /kubepark binary path inside the operator image.
	agentDir = "/opt/kubepark"

	annotationSafeToEvict = "cluster-autoscaler.kubernetes.io/safe-to-evict"

	volumeHome  = "home"
	volumeAgent = "kubepark-bin"
)

// Options are operator-level knobs that shape sandbox pods.
type Options struct {
	// AgentImage is the kubepark server image used by the agent-install
	// init container (normally the operator's own image).
	AgentImage string
	// PriorityClassName is set on sandbox pods when non-empty.
	PriorityClassName string
}

// Names derived from the sandbox name. Kept together so the controller and
// tests agree on them.
func PodName(sandbox string) string     { return "kubepark-sb-" + sandbox }
func PVCName(sandbox string) string     { return "kubepark-home-" + sandbox }
func HostKeyName(sandbox string) string { return "kubepark-hostkey-" + sandbox }
func NetPolName(sandbox string) string  { return "kubepark-sb-" + sandbox }

// Labels returns the canonical label set for resources owned by a sandbox.
func Labels(sb *kubeparkv1alpha1.Sandbox) map[string]string {
	return map[string]string{
		LabelSandbox:    sb.Name,
		LabelSandboxUID: string(sb.UID),
		LabelComponent:  ComponentSandbox,
		LabelManagedBy:  "kubepark",
	}
}

// TemplateHash returns a stable short hash of the template spec, used to
// pin the template snapshot a pod was built from (template edits must never
// restart running pods).
func TemplateHash(spec *kubeparkv1alpha1.SandboxTemplateSpec) string {
	raw, err := json.Marshal(spec)
	if err != nil {
		// A SandboxTemplateSpec always marshals; guard for completeness.
		return "unhashable"
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:12]
}

// BuildPod renders the executor pod for the sandbox from the given template
// snapshot.
func BuildPod(sb *kubeparkv1alpha1.Sandbox, tpl *kubeparkv1alpha1.SandboxTemplate, opts Options) *corev1.Pod {
	homeClaim := PVCName(sb.Name)
	if sb.Spec.Home != nil && sb.Spec.Home.ExistingClaim != "" {
		homeClaim = sb.Spec.Home.ExistingClaim
	}

	runAsUser := int64(1000)
	if tpl.Spec.RunAsUser != nil {
		runAsUser = *tpl.Spec.RunAsUser
	}

	// The agent is PID 1. A non-empty template command runs as its child;
	// with no command the agent idles and spawns shells per connection.
	command := []string{agentDir + "/agent"}
	var args []string
	if len(tpl.Spec.Command) > 0 {
		args = append([]string{"--"}, tpl.Spec.Command...)
	}

	env := append([]corev1.EnvVar{
		{Name: "HOME", Value: HomeMountPath},
		{Name: "KUBEPARK_SANDBOX", Value: sb.Name},
		{Name: "KUBEPARK_NAMESPACE", Value: sb.Namespace},
		{Name: "KUBEPARK_OWNER", Value: sb.Spec.Owner.Name},
	}, tpl.Spec.Env...)

	ports := make([]corev1.ContainerPort, 0, 1+len(sb.Spec.ExposedPorts))
	ports = append(ports, corev1.ContainerPort{Name: "ssh", ContainerPort: AgentPort, Protocol: corev1.ProtocolTCP})
	for _, p := range sb.Spec.ExposedPorts {
		ports = append(ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	containerSecurity := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PodName(sb.Name),
			Namespace: sb.Namespace,
			Labels:    Labels(sb),
			Annotations: map[string]string{
				annotationSafeToEvict: "false",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			// Stable hostname: the shell prompt names the sandbox, not a
			// generated pod name.
			Hostname:                      sb.Name,
			AutomountServiceAccountToken:  ptr.To(false),
			TerminationGracePeriodSeconds: ptr.To(int64(30)),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(runAsUser),
				FSGroup:      ptr.To(runAsUser),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			InitContainers: []corev1.Container{{
				Name:            "agent-install",
				Image:           opts.AgentImage,
				Args:            []string{"agent", "install", agentDir},
				SecurityContext: containerSecurity,
				VolumeMounts: []corev1.VolumeMount{
					{Name: volumeAgent, MountPath: agentDir},
				},
			}},
			Containers: []corev1.Container{{
				Name:            "sandbox",
				Image:           tpl.Spec.Image,
				Command:         command,
				Args:            args,
				Env:             env,
				Ports:           ports,
				Resources:       tpl.Spec.Resources,
				SecurityContext: containerSecurity,
				VolumeMounts: []corev1.VolumeMount{
					{Name: volumeHome, MountPath: HomeMountPath},
					{Name: volumeAgent, MountPath: agentDir, ReadOnly: true},
				},
			}},
			Volumes: []corev1.Volume{
				{
					Name: volumeHome,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: homeClaim,
						},
					},
				},
				{
					Name: volumeAgent,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	if opts.PriorityClassName != "" {
		pod.Spec.PriorityClassName = opts.PriorityClassName
	}
	if tpl.Spec.IsolationLevel == kubeparkv1alpha1.IsolationStrong && tpl.Spec.RuntimeClassName != nil {
		pod.Spec.RuntimeClassName = tpl.Spec.RuntimeClassName
	}
	return pod
}

// BuildPVC renders the home PVC for a sandbox that does not use an existing
// claim.
func BuildPVC(sb *kubeparkv1alpha1.Sandbox, tpl *kubeparkv1alpha1.SandboxTemplate) *corev1.PersistentVolumeClaim {
	size := tpl.Spec.HomeSize
	storageClass := tpl.Spec.StorageClassName
	if sb.Spec.Home != nil {
		if sb.Spec.Home.Size != nil {
			size = *sb.Spec.Home.Size
		}
		if sb.Spec.Home.StorageClassName != nil {
			storageClass = sb.Spec.Home.StorageClassName
		}
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCName(sb.Name),
			Namespace: sb.Namespace,
			Labels:    Labels(sb),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}
}

// FQDNPrincipals returns the host-certificate principals clients may use to
// reach this sandbox through the gateway.
func FQDNPrincipals(sb *kubeparkv1alpha1.Sandbox) []string {
	return []string{fmt.Sprintf("%s.%s", sb.Name, sb.Namespace)}
}
