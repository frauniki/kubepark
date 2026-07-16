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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

// ComponentGateway is the LabelComponent value on gateway pods; sandbox
// ingress is restricted to pods carrying it.
const ComponentGateway = "gateway"

// APIServerEndpoint is one resolved address of the Kubernetes API server.
type APIServerEndpoint struct {
	IP   string
	Port int32
}

// NetPolOptions carries the cluster-specific inputs of the sandbox
// NetworkPolicy.
type NetPolOptions struct {
	// GatewayNamespace is where gateway pods run (ingress allowance).
	GatewayNamespace string
	// APIServerEndpoints are the resolved kubernetes.default endpoints.
	// A static egress rule cannot express "the API server" portably, so
	// the controller resolves the Endpoints object and keeps this fresh.
	APIServerEndpoints []APIServerEndpoint
}

// BuildNetworkPolicy renders the per-sandbox policy: default-deny both
// directions, ingress only from the gateway, egress to DNS, the API server
// and whatever the template allows.
func BuildNetworkPolicy(sb *kubeparkv1alpha1.Sandbox, tpl *kubeparkv1alpha1.SandboxTemplate, opts NetPolOptions) *networkingv1.NetworkPolicy {
	protoTCP := corev1.ProtocolTCP
	protoUDP := corev1.ProtocolUDP

	// Ingress: gateway pods only, to the agent port and any exposed ports.
	gatewayPeer := networkingv1.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				corev1.LabelMetadataName: opts.GatewayNamespace,
			},
		},
		PodSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				LabelComponent: ComponentGateway,
			},
		},
	}
	ingressPorts := make([]networkingv1.NetworkPolicyPort, 0, 1+len(sb.Spec.ExposedPorts))
	ingressPorts = append(ingressPorts, networkingv1.NetworkPolicyPort{Protocol: &protoTCP, Port: ptrIntStr(AgentPort)})
	for _, p := range sb.Spec.ExposedPorts {
		ingressPorts = append(ingressPorts, networkingv1.NetworkPolicyPort{
			Protocol: &protoTCP, Port: ptrIntStr(p.Port),
		})
	}

	// Egress: DNS to kube-dns.
	dnsPort := intstr.FromInt32(53)
	dnsRule := networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					corev1.LabelMetadataName: "kube-system",
				},
			},
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"k8s-app": "kube-dns"},
			},
		}},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &protoUDP, Port: &dnsPort},
			{Protocol: &protoTCP, Port: &dnsPort},
		},
	}

	egress := make([]networkingv1.NetworkPolicyEgressRule, 0, 1+len(opts.APIServerEndpoints)+len(tpl.Spec.Egress))
	egress = append(egress, dnsRule)

	// Egress: the API server, resolved to concrete endpoints. Without this
	// the injected ServiceAccount credentials are useless (and on CNIs
	// that enforce NetworkPolicy, kubectl inside the sandbox hangs).
	for _, ep := range opts.APIServerEndpoints {
		port := intstr.FromInt32(ep.Port)
		egress = append(egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				IPBlock: &networkingv1.IPBlock{CIDR: ep.IP + "/32"},
			}},
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: &protoTCP, Port: &port},
			},
		})
	}

	// Egress: template vocabulary, verbatim.
	for _, rule := range tpl.Spec.Egress {
		egress = append(egress, networkingv1.NetworkPolicyEgressRule{
			To:    rule.To,
			Ports: rule.Ports,
		})
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetPolName(sb.Name),
			Namespace: sb.Namespace,
			Labels:    Labels(sb),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{LabelSandboxUID: string(sb.UID)},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From:  []networkingv1.NetworkPolicyPeer{gatewayPeer},
				Ports: ingressPorts,
			}},
			Egress: egress,
		},
	}
}

func ptrIntStr(port int32) *intstr.IntOrString {
	v := intstr.FromInt32(port)
	return &v
}
