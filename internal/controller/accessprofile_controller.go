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
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

// AccessProfileReconciler maintains the shared Role a profile grants into
// each of its namespaces and validates that those namespaces exist.
type AccessProfileReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubepark.dev,resources=accessprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepark.dev,resources=accessprofiles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubepark.dev,resources=accessprofiles/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete;escalate;bind
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile validates the profile, syncs its shared Roles, and re-queues
// the sandboxes that reference it.
func (r *AccessProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var profile kubeparkv1alpha1.AccessProfile
	if err := r.Get(ctx, req.NamespacedName, &profile); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !profile.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &profile)
	}

	if !controllerutil.ContainsFinalizer(&profile, AccessProfileFinalizer) {
		controllerutil.AddFinalizer(&profile, AccessProfileFinalizer)
		if err := r.Update(ctx, &profile); err != nil {
			return ctrl.Result{}, err
		}
	}

	missing, err := r.syncRoles(ctx, &profile)
	if err != nil {
		log.Error(err, "Failed to sync profile roles")
		return ctrl.Result{}, err
	}

	// Re-queue referencing sandboxes so allowedNamespaces / rule changes
	// take effect on their RoleBindings.
	r.requeueSandboxes(ctx, profile.Name)

	profile.Status.ObservedGeneration = profile.Generation
	if len(missing) == 0 {
		meta.SetStatusCondition(&profile.Status.Conditions, metav1.Condition{
			Type: kubeparkv1alpha1.ConditionValid, Status: metav1.ConditionTrue,
			Reason: kubeparkv1alpha1.ReasonValid, Message: "all grant namespaces exist",
			ObservedGeneration: profile.Generation,
		})
	} else {
		meta.SetStatusCondition(&profile.Status.Conditions, metav1.Condition{
			Type: kubeparkv1alpha1.ConditionValid, Status: metav1.ConditionFalse,
			Reason:             kubeparkv1alpha1.ReasonMissingNamespace,
			Message:            fmt.Sprintf("missing namespaces: %s (other grants applied)", strings.Join(missing, ", ")),
			ObservedGeneration: profile.Generation,
		})
	}
	return ctrl.Result{}, client.IgnoreNotFound(r.Status().Update(ctx, &profile))
}

// syncRoles reconciles one Role per existing grant namespace holding the
// union of that namespace's rules, and garbage-collects Roles in
// namespaces the profile no longer grants into. It returns the missing
// namespaces so the caller can surface a partial-apply status.
func (r *AccessProfileReconciler) syncRoles(ctx context.Context, profile *kubeparkv1alpha1.AccessProfile) ([]string, error) {
	// Union rules per namespace.
	rulesByNS := map[string][]rbacv1.PolicyRule{}
	for _, grant := range profile.Spec.Grants {
		for _, ns := range grant.Namespaces {
			rulesByNS[ns] = append(rulesByNS[ns], grant.Rules...)
		}
	}

	var missing []string
	applied := map[string]struct{}{}
	for ns, rules := range rulesByNS {
		exists, err := r.namespaceExists(ctx, ns)
		if err != nil {
			return nil, err
		}
		if !exists {
			missing = append(missing, ns)
			continue
		}
		if err := r.applyRole(ctx, profile, ns, rules); err != nil {
			return nil, err
		}
		applied[ns] = struct{}{}
	}

	// GC Roles in namespaces no longer applied.
	var roles rbacv1.RoleList
	if err := r.List(ctx, &roles, client.MatchingLabels{LabelProfile: profile.Name}); err != nil {
		return nil, err
	}
	for i := range roles.Items {
		role := &roles.Items[i]
		if _, ok := applied[role.Namespace]; ok {
			continue
		}
		if err := r.Delete(ctx, role); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	slices.Sort(missing)
	return missing, nil
}

func (r *AccessProfileReconciler) applyRole(ctx context.Context, profile *kubeparkv1alpha1.AccessProfile, ns string, rules []rbacv1.PolicyRule) error {
	desired := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      profileRoleName(profile.Name),
			Namespace: ns,
			Labels: map[string]string{
				LabelProfile:                   profile.Name,
				"app.kubernetes.io/managed-by": ManagedByValue,
			},
		},
		Rules: rules,
	}
	var existing rbacv1.Role
	err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		return client.IgnoreAlreadyExists(r.Create(ctx, desired))
	}
	if err != nil {
		return err
	}
	if !equality(existing.Rules, desired.Rules) {
		existing.Rules = desired.Rules
		if existing.Labels == nil {
			existing.Labels = desired.Labels
		}
		return r.Update(ctx, &existing)
	}
	return nil
}

// finalize deletes every Role the profile created and re-queues the
// sandboxes that referenced it so they flip to RBACReady=False.
func (r *AccessProfileReconciler) finalize(ctx context.Context, profile *kubeparkv1alpha1.AccessProfile) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(profile, AccessProfileFinalizer) {
		return ctrl.Result{}, nil
	}
	var roles rbacv1.RoleList
	if err := r.List(ctx, &roles, client.MatchingLabels{LabelProfile: profile.Name}); err != nil {
		return ctrl.Result{}, err
	}
	for i := range roles.Items {
		if err := r.Delete(ctx, &roles.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}
	r.requeueSandboxes(ctx, profile.Name)

	controllerutil.RemoveFinalizer(profile, AccessProfileFinalizer)
	return ctrl.Result{}, r.Update(ctx, profile)
}

func (r *AccessProfileReconciler) namespaceExists(ctx context.Context, name string) (bool, error) {
	var ns corev1.Namespace
	err := r.Get(ctx, types.NamespacedName{Name: name}, &ns)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// requeueSandboxes triggers reconciliation of every sandbox referencing the
// profile by touching nothing but relying on the sandbox watch mapping; it
// is best-effort (errors are only logged).
func (r *AccessProfileReconciler) requeueSandboxes(ctx context.Context, profile string) {
	log := logf.FromContext(ctx)
	var sandboxes kubeparkv1alpha1.SandboxList
	if err := r.List(ctx, &sandboxes, client.MatchingFields{indexSandboxAccessProfile: profile}); err != nil {
		log.Error(err, "Failed to list sandboxes for profile requeue", "profile", profile)
		return
	}
	for i := range sandboxes.Items {
		sb := &sandboxes.Items[i]
		// A no-op annotation bump forces the sandbox controller to
		// re-evaluate its RBAC without another watch wiring.
		patch := client.MergeFrom(sb.DeepCopy())
		if sb.Annotations == nil {
			sb.Annotations = map[string]string{}
		}
		sb.Annotations["kubepark.dev/profile-generation"] = fmt.Sprintf("%d-%s", sb.Generation, profile)
		if err := r.Patch(ctx, sb, patch); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to nudge sandbox", "sandbox", sb.Name)
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeparkv1alpha1.AccessProfile{}).
		Watches(&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.profilesForNamespace)).
		Named("accessprofile").
		Complete(r)
}

// profilesForNamespace re-validates profiles when a namespace appears
// (turning a MissingNamespace into an applied grant).
func (r *AccessProfileReconciler) profilesForNamespace(ctx context.Context, ns client.Object) []ctrl.Request {
	var profiles kubeparkv1alpha1.AccessProfileList
	if err := r.List(ctx, &profiles); err != nil {
		return nil
	}
	var reqs []ctrl.Request
	for i := range profiles.Items {
		if slices.Contains(grantNamespaces(&profiles.Items[i]), ns.GetName()) {
			reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{Name: profiles.Items[i].Name}})
		}
	}
	return reqs
}
