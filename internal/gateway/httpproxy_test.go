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

package gateway

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

func TestAuthorizedForPort(t *testing.T) {
	sb := &kubeparkv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: sbName, Namespace: nsAlice},
		Spec:       kubeparkv1alpha1.SandboxSpec{Owner: kubeparkv1alpha1.OwnerSpec{Name: "alice@example.com"}},
	}
	port := &kubeparkv1alpha1.ExposedPort{
		Name:          "jupyter",
		AllowedUsers:  []string{"bob@example.com"},
		AllowedGroups: []string{"team-ml"},
	}

	// Owner is always allowed.
	if !authorizedForPort(sb, port, "alice@example.com", nil) {
		t.Error("owner must be authorized")
	}
	// A different authenticated user is NOT allowed by authentication alone.
	if authorizedForPort(sb, port, "mallory@example.com", nil) {
		t.Error("a non-owner without an allow entry must be forbidden")
	}
	// Explicit allowedUsers entry.
	if !authorizedForPort(sb, port, "bob@example.com", nil) {
		t.Error("allowedUsers entry must be authorized")
	}
	// Explicit allowedGroups entry.
	if !authorizedForPort(sb, port, "carol@example.com", []string{"team-ml"}) {
		t.Error("allowedGroups membership must be authorized")
	}
	// Wrong group is forbidden.
	if authorizedForPort(sb, port, "carol@example.com", []string{"team-other"}) {
		t.Error("a non-matching group must be forbidden")
	}
}
