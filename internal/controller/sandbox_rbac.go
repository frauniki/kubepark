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
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/controller/podspec"
)

// rbacResult reports the outcome of RBAC reconciliation to the caller.
type rbacResult struct {
	// ServiceAccount is the SA the pod should run as (empty when the
	// sandbox has no AccessProfile).
	ServiceAccount string
	// Ready is false when the profile is missing or not permitted; the
	// caller must not start the pod with stale credentials.
	Ready bool
}

// reconcileRBAC translates the referenced AccessProfile into a per-sandbox
// ServiceAccount plus a RoleBinding in every grant namespace, and reflects
// the outcome into the RBACReady condition.
func (r *SandboxReconciler) reconcileRBAC(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, status *kubeparkv1alpha1.SandboxStatus) (rbacResult, error) {
	if sb.Spec.AccessProfile == "" {
		// No profile: the sandbox gets no Kubernetes credentials. Clean up
		// anything a previously-set profile left behind.
		if err := r.gcRBAC(ctx, sb, nil); err != nil {
			return rbacResult{}, err
		}
		status.ServiceAccountName = ""
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionRBACReady, metav1.ConditionTrue,
			kubeparkv1alpha1.ReasonRunning, "no access profile; sandbox has no cluster credentials")
		return rbacResult{Ready: true}, nil
	}

	var profile kubeparkv1alpha1.AccessProfile
	err := r.Get(ctx, types.NamespacedName{Name: sb.Spec.AccessProfile}, &profile)
	if apierrors.IsNotFound(err) {
		if gcErr := r.gcRBAC(ctx, sb, nil); gcErr != nil {
			return rbacResult{}, gcErr
		}
		status.ServiceAccountName = ""
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionRBACReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProfileDeleted,
			fmt.Sprintf("AccessProfile %q not found", sb.Spec.AccessProfile))
		return rbacResult{}, nil
	}
	if err != nil {
		return rbacResult{}, err
	}

	// Referencing, not creation, is the escalation surface: refuse a
	// profile whose allowedNamespaces does not include this sandbox.
	if !slices.Contains(profile.Spec.AllowedNamespaces, sb.Namespace) {
		if gcErr := r.gcRBAC(ctx, sb, nil); gcErr != nil {
			return rbacResult{}, gcErr
		}
		status.ServiceAccountName = ""
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionRBACReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProfileNotPermitted,
			fmt.Sprintf("namespace %q is not in AccessProfile %q allowedNamespaces", sb.Namespace, profile.Name))
		return rbacResult{}, nil
	}

	// ServiceAccount, annotated so apiserver audit logs join back to the
	// owner and the profile.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName(sb.Name),
			Namespace: sb.Namespace,
			Labels:    podspec.Labels(sb),
			Annotations: map[string]string{
				"kubepark.dev/owner":          sb.Spec.Owner.Name,
				"kubepark.dev/access-profile": profile.Name,
			},
		},
	}
	if err := r.ensureOwned(ctx, sb, sa); err != nil {
		return rbacResult{}, err
	}

	// One RoleBinding per grant namespace binding the SA to the shared
	// profile Role (which the AccessProfile controller maintains).
	desiredNamespaces := grantNamespaces(&profile)
	for _, ns := range desiredNamespaces {
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName(sb.Name),
				Namespace: ns,
				Labels: map[string]string{
					podspec.LabelSandboxUID: string(sb.UID),
					LabelProfile:            profile.Name,
					podspec.LabelManagedBy:  ManagedByValue,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     profileRoleName(profile.Name),
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName(sb.Name),
				Namespace: sb.Namespace,
			}},
		}
		if err := r.applyRoleBinding(ctx, rb); err != nil {
			return rbacResult{}, err
		}
	}

	// Remove RoleBindings in namespaces no longer granted.
	if err := r.gcRBAC(ctx, sb, desiredNamespaces); err != nil {
		return rbacResult{}, err
	}

	status.ServiceAccountName = sa.Name
	r.setCondition(sb, status, kubeparkv1alpha1.ConditionRBACReady, metav1.ConditionTrue,
		kubeparkv1alpha1.ReasonRunning,
		fmt.Sprintf("bound to AccessProfile %q in %d namespace(s)", profile.Name, len(desiredNamespaces)))
	return rbacResult{ServiceAccount: sa.Name, Ready: true}, nil
}

// ensureOwned creates a namespace-local object owned by the sandbox if it
// does not already exist.
func (r *SandboxReconciler) ensureOwned(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, obj client.Object) error {
	if err := controllerSetOwner(sb, obj, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// applyRoleBinding creates the RoleBinding, or replaces it if the roleRef
// drifted (RoleRef is immutable, so a change means delete + recreate).
func (r *SandboxReconciler) applyRoleBinding(ctx context.Context, desired *rbacv1.RoleBinding) error {
	var existing rbacv1.RoleBinding
	err := r.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		return client.IgnoreAlreadyExists(r.Create(ctx, desired))
	}
	if err != nil {
		return err
	}
	if existing.RoleRef != desired.RoleRef {
		if err := r.Delete(ctx, &existing); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return client.IgnoreAlreadyExists(r.Create(ctx, desired))
	}
	if !equality(existing.Subjects, desired.Subjects) {
		existing.Subjects = desired.Subjects
		return r.Update(ctx, &existing)
	}
	return nil
}

// gcRBAC deletes RoleBindings for this sandbox whose namespace is not in
// keep (nil keep removes all of them). The SA is namespace-local and is
// left to the finalizer / owner reference.
func (r *SandboxReconciler) gcRBAC(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, keep []string) error {
	var bindings rbacv1.RoleBindingList
	if err := r.List(ctx, &bindings, client.MatchingLabels{podspec.LabelSandboxUID: string(sb.UID)}); err != nil {
		return err
	}
	for i := range bindings.Items {
		rb := &bindings.Items[i]
		if keep != nil && slices.Contains(keep, rb.Namespace) {
			continue
		}
		if err := r.Delete(ctx, rb); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// grantNamespaces returns the deduplicated set of namespaces a profile
// grants into.
func grantNamespaces(profile *kubeparkv1alpha1.AccessProfile) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, g := range profile.Spec.Grants {
		for _, ns := range g.Namespaces {
			if _, ok := seen[ns]; ok {
				continue
			}
			seen[ns] = struct{}{}
			out = append(out, ns)
		}
	}
	return out
}
