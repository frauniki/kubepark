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

const (
	// LabelProfile marks Roles/RoleBindings produced from an AccessProfile.
	LabelProfile = "kubepark.dev/profile"
	// ManagedByValue is the app.kubernetes.io/managed-by value on all
	// kubepark-created objects.
	ManagedByValue = "kubepark"

	// AccessProfileFinalizer guards cleanup of a profile's shared Roles.
	AccessProfileFinalizer = "kubepark.dev/finalizer"

	// indexSandboxAccessProfile indexes sandboxes by their profile ref.
	indexSandboxAccessProfile = ".spec.accessProfile"
)

// saName is the per-sandbox ServiceAccount name carrying AccessProfile
// grants and the owner identity annotation.
func saName(sandbox string) string { return "kubepark-sb-" + sandbox }

// profileRoleName is the shared Role name a profile reconciles into each of
// its grant namespaces.
func profileRoleName(profile string) string { return "kubepark-ap-" + profile }

// roleBindingName is the per-sandbox RoleBinding created in each grant
// namespace binding the sandbox SA to the profile Role.
func roleBindingName(sandbox string) string { return "kubepark-sb-" + sandbox }
