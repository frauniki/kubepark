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

package controller

import (
	"golang.org/x/crypto/ssh"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

// controllerSetOwner sets the sandbox as the controller owner of a
// namespace-local object.
func controllerSetOwner(sb *kubeparkv1alpha1.Sandbox, obj client.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(sb, obj, scheme)
}

// equality is semantic deep-equality (resource quantities compare by value,
// nil and empty collections compare equal).
func equality(a, b any) bool {
	return apiequality.Semantic.DeepEqual(a, b)
}

// marshalCert renders a certificate in authorized_keys format (the
// *-cert.pub file layout OpenSSH expects).
func marshalCert(cert *ssh.Certificate) []byte {
	return ssh.MarshalAuthorizedKey(cert)
}
