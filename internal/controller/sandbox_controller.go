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
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	"github.com/frauniki/kubepark/internal/controller/podspec"
	"github.com/frauniki/kubepark/internal/sshca"
)

const (
	// SandboxFinalizer guards cleanup of everything a sandbox owns across
	// namespaces (and the retain-policy handling of its home PVC).
	SandboxFinalizer = "kubepark.dev/finalizer"

	// LabelOrphanedHome marks retained home PVCs whose Sandbox is gone.
	LabelOrphanedHome = "kubepark.dev/orphaned-home"

	// indexSandboxTemplate indexes sandboxes by their template reference.
	indexSandboxTemplate = ".spec.template"
	// indexSandboxExistingClaim indexes sandboxes by home.existingClaim.
	indexSandboxExistingClaim = ".spec.home.existingClaim"

	hostCertValidity = 10 * 365 * 24 * time.Hour
)

// SandboxReconciler reconciles a Sandbox object.
type SandboxReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// AgentImage is the image used for the agent-install init container.
	AgentImage string
	// PriorityClassName is set on sandbox pods when non-empty.
	PriorityClassName string
	// GatewayNamespace is where gateway pods run; defaults to the operator
	// namespace.
	GatewayNamespace string
}

// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxtemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxsessions,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxsessions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepark.dev,resources=accessprofiles,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete;bind

// Reconcile drives the sandbox state machine. The pod is a disposable
// executor: PVC, host key and (in later milestones) RBAC survive it.
func (r *SandboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sb kubeparkv1alpha1.Sandbox
	if err := r.Get(ctx, req.NamespacedName, &sb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !sb.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &sb)
	}

	if !controllerutil.ContainsFinalizer(&sb, SandboxFinalizer) {
		controllerutil.AddFinalizer(&sb, SandboxFinalizer)
		if err := r.Update(ctx, &sb); err != nil {
			return ctrl.Result{}, err
		}
	}

	status := *sb.Status.DeepCopy()
	status.ObservedGeneration = sb.Generation
	result, err := r.reconcileSandbox(ctx, &sb, &status)
	if err != nil {
		log.Error(err, "Reconcile failed")
	}
	if updErr := r.patchStatus(ctx, &sb, &status); updErr != nil {
		if err == nil {
			err = updErr
		}
	}
	return result, err
}

// reconcileSandbox computes desired children and the next status. It writes
// children to the cluster but only mutates status in memory; the caller
// persists it once.
func (r *SandboxReconciler) reconcileSandbox(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, status *kubeparkv1alpha1.SandboxStatus) (ctrl.Result, error) {
	// Resolve the template; without it nothing can be provisioned.
	var tpl kubeparkv1alpha1.SandboxTemplate
	if err := r.Get(ctx, types.NamespacedName{Name: sb.Spec.Template}, &tpl); err != nil {
		if apierrors.IsNotFound(err) {
			status.Phase = kubeparkv1alpha1.SandboxPhasePending
			r.setCondition(sb, status, kubeparkv1alpha1.ConditionReady, metav1.ConditionFalse,
				kubeparkv1alpha1.ReasonInvalidRef,
				fmt.Sprintf("SandboxTemplate %q not found", sb.Spec.Template))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Home volume, including the shared-claim guard.
	requeue, err := r.reconcileHome(ctx, sb, status)
	if err != nil || requeue != nil {
		return valueOr(requeue), err
	}

	// Host key (stable across suspend/resume) and network policy.
	if err := r.reconcileHostKey(ctx, sb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileNetworkPolicy(ctx, sb, &tpl); err != nil {
		return ctrl.Result{}, err
	}

	// Access profile -> ServiceAccount + RBAC. A not-permitted or missing
	// profile blocks the pod so it never runs with stale credentials.
	rbac, err := r.reconcileRBAC(ctx, sb, status)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !rbac.Ready {
		status.Phase = kubeparkv1alpha1.SandboxPhasePending
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProvisioning, "waiting on access profile")
		return ctrl.Result{}, nil
	}

	// Desired-state machine.
	currentHash := podspec.TemplateHash(&tpl.Spec)
	if sb.Spec.DesiredState == kubeparkv1alpha1.DesiredStateStopped {
		return r.suspend(ctx, sb, status)
	}
	return r.run(ctx, sb, &tpl, currentHash, rbac.ServiceAccount, status)
}

// reconcileHome ensures the home PVC. A non-nil result means "stop this
// reconcile and requeue accordingly".
func (r *SandboxReconciler) reconcileHome(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, status *kubeparkv1alpha1.SandboxStatus) (*ctrl.Result, error) {
	claim := podspec.PVCName(sb.Name)
	if sb.Spec.Home != nil && sb.Spec.Home.ExistingClaim != "" {
		claim = sb.Spec.Home.ExistingClaim

		conflict, err := r.claimConflict(ctx, sb, claim)
		if err != nil {
			return nil, err
		}
		if conflict != "" {
			status.Phase = kubeparkv1alpha1.SandboxPhasePending
			r.setCondition(sb, status, kubeparkv1alpha1.ConditionHomeReady, metav1.ConditionFalse,
				kubeparkv1alpha1.ReasonClaimInUse,
				fmt.Sprintf("claim %q is in use by sandbox %q", claim, conflict))
			return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		var pvc corev1.PersistentVolumeClaim
		if err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: claim}, &pvc); err != nil {
			if apierrors.IsNotFound(err) {
				status.Phase = kubeparkv1alpha1.SandboxPhasePending
				r.setCondition(sb, status, kubeparkv1alpha1.ConditionHomeReady, metav1.ConditionFalse,
					kubeparkv1alpha1.ReasonInvalidRef,
					fmt.Sprintf("existing claim %q not found", claim))
				return &ctrl.Result{}, nil
			}
			return nil, err
		}
	} else {
		var tpl kubeparkv1alpha1.SandboxTemplate
		if err := r.Get(ctx, types.NamespacedName{Name: sb.Spec.Template}, &tpl); err != nil {
			return nil, err
		}
		var pvc corev1.PersistentVolumeClaim
		err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: claim}, &pvc)
		if apierrors.IsNotFound(err) {
			// No owner reference on purpose: the PVC's lifecycle is
			// independent of the Sandbox; the finalizer applies the retain
			// policy explicitly.
			if err := r.Create(ctx, podspec.BuildPVC(sb, &tpl)); err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}

	status.PVCName = claim
	r.setCondition(sb, status, kubeparkv1alpha1.ConditionHomeReady, metav1.ConditionTrue,
		kubeparkv1alpha1.ReasonProvisioning, "home volume ready")
	return nil, nil
}

// claimConflict returns the name of another non-suspended sandbox using the
// same claim that wins the deterministic tie-break (older creation wins;
// UID breaks exact ties).
func (r *SandboxReconciler) claimConflict(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, claim string) (string, error) {
	var others kubeparkv1alpha1.SandboxList
	if err := r.List(ctx, &others, client.InNamespace(sb.Namespace),
		client.MatchingFields{indexSandboxExistingClaim: claim}); err != nil {
		return "", err
	}
	candidates := others.Items

	// A generated claim name kubepark-home-<x> may also be owned by
	// sandbox <x> without existingClaim set; include it.
	if owner, ok := ownerOfGeneratedClaim(claim); ok && owner != sb.Name {
		var ownerSb kubeparkv1alpha1.Sandbox
		err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: owner}, &ownerSb)
		if err == nil {
			candidates = append(candidates, ownerSb)
		} else if !apierrors.IsNotFound(err) {
			return "", err
		}
	}

	for i := range candidates {
		other := &candidates[i]
		if other.UID == sb.UID || !other.DeletionTimestamp.IsZero() {
			continue
		}
		if other.Spec.DesiredState == kubeparkv1alpha1.DesiredStateStopped &&
			other.Status.Phase == kubeparkv1alpha1.SandboxPhaseSuspended {
			continue
		}
		if wins(other, sb) {
			return other.Name, nil
		}
	}
	return "", nil
}

// wins reports whether a beats b in the claim tie-break.
func wins(a, b *kubeparkv1alpha1.Sandbox) bool {
	at, bt := a.CreationTimestamp, b.CreationTimestamp
	if !at.Equal(&bt) {
		return at.Before(&bt)
	}
	return string(a.UID) < string(b.UID)
}

func ownerOfGeneratedClaim(claim string) (string, bool) {
	const prefix = "kubepark-home-"
	if len(claim) > len(prefix) && claim[:len(prefix)] == prefix {
		return claim[len(prefix):], true
	}
	return "", false
}

// reconcileHostKey creates the per-sandbox host key Secret once, signed by
// the host CA, so the host identity is stable across suspend/resume.
func (r *SandboxReconciler) reconcileHostKey(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) error {
	name := podspec.HostKeyName(sb.Name)
	var existing corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: name}, &existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	caSecret, err := EnsureCASecret(ctx, r.Client, OperatorNamespace())
	if err != nil {
		return err
	}
	hostCA, err := sshca.ParseSigner(caSecret.Data[KeyHostCAPrivate])
	if err != nil {
		return fmt.Errorf("parse host CA: %w", err)
	}

	key, err := sshca.GenerateKeyPair("kubepark-host-" + sb.Name)
	if err != nil {
		return err
	}
	hostPub, err := sshca.ParsePublicKey(key.PublicAuthorized)
	if err != nil {
		return err
	}
	cert, err := sshca.SignHostCert(hostCA, hostPub, podspec.FQDNPrincipals(sb), hostCertValidity, time.Now())
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sb.Namespace,
			Labels:    podspec.Labels(sb),
		},
		Data: map[string][]byte{
			"ssh_host_ed25519_key":          key.PrivatePEM,
			"ssh_host_ed25519_key.pub":      key.PublicAuthorized,
			"ssh_host_ed25519_key-cert.pub": marshalCert(cert),
		},
	}
	if err := controllerutil.SetControllerReference(sb, secret, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// reconcileNetworkPolicy keeps the default-deny policy (with built-in DNS
// and API-server egress) in sync.
func (r *SandboxReconciler) reconcileNetworkPolicy(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, tpl *kubeparkv1alpha1.SandboxTemplate) error {
	endpoints, err := r.apiServerEndpoints(ctx)
	if err != nil {
		return err
	}
	desired := podspec.BuildNetworkPolicy(sb, tpl, podspec.NetPolOptions{
		GatewayNamespace:   r.gatewayNamespace(),
		APIServerEndpoints: endpoints,
	})
	if err := controllerutil.SetControllerReference(sb, desired, r.Scheme); err != nil {
		return err
	}

	var existing networkingv1.NetworkPolicy
	err = r.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		return client.IgnoreAlreadyExists(r.Create(ctx, desired))
	}
	if err != nil {
		return err
	}
	if !equality(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		return r.Update(ctx, &existing)
	}
	return nil
}

// apiServerEndpoints resolves kubernetes.default from its EndpointSlices.
// Static egress rules cannot express the API server portably; the
// controller re-renders the policy whenever these endpoints change.
func (r *SandboxReconciler) apiServerEndpoints(ctx context.Context) ([]podspec.APIServerEndpoint, error) {
	var slices discoveryv1.EndpointSliceList
	if err := r.List(ctx, &slices, client.InNamespace(metav1.NamespaceDefault),
		client.MatchingLabels{discoveryv1.LabelServiceName: "kubernetes"}); err != nil {
		return nil, fmt.Errorf("resolve kubernetes.default endpoint slices: %w", err)
	}
	var out []podspec.APIServerEndpoint
	for _, slice := range slices.Items {
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			for _, addr := range ep.Addresses {
				for _, port := range slice.Ports {
					if port.Port == nil {
						continue
					}
					out = append(out, podspec.APIServerEndpoint{IP: addr, Port: *port.Port})
				}
			}
		}
	}
	return out, nil
}

// suspend deletes the pod while keeping PVC, host key, RBAC and route.
func (r *SandboxReconciler) suspend(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, status *kubeparkv1alpha1.SandboxStatus) (ctrl.Result, error) {
	// The gateway must not dial a terminating pod (R2-L-B).
	status.PodIP = ""

	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: podspec.PodName(sb.Name)}, &pod)
	if apierrors.IsNotFound(err) {
		status.Phase = kubeparkv1alpha1.SandboxPhaseSuspended
		status.PodName = ""
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionPodReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonSuspended, "sandbox is suspended")
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonSuspended, "sandbox is suspended")
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	status.Phase = kubeparkv1alpha1.SandboxPhaseSuspending
	if pod.DeletionTimestamp.IsZero() {
		if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// run ensures the executor pod exists and reflects pod state into status.
func (r *SandboxReconciler) run(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, tpl *kubeparkv1alpha1.SandboxTemplate, currentHash, serviceAccount string, status *kubeparkv1alpha1.SandboxStatus) (ctrl.Result, error) {
	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: podspec.PodName(sb.Name)}, &pod)
	if apierrors.IsNotFound(err) {
		resuming := status.Phase == kubeparkv1alpha1.SandboxPhaseSuspended ||
			status.Phase == kubeparkv1alpha1.SandboxPhaseSuspending ||
			status.Phase == kubeparkv1alpha1.SandboxPhaseResuming

		desired := podspec.BuildPod(sb, tpl, podspec.Options{
			AgentImage:         r.AgentImage,
			PriorityClassName:  r.PriorityClassName,
			ServiceAccountName: serviceAccount,
		})
		if err := controllerutil.SetControllerReference(sb, desired, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		// The pod is always built from the CURRENT template: template
		// changes apply on (re)provisioning, never by restarting a
		// running pod.
		status.TemplateHash = currentHash
		status.PodName = desired.Name
		if resuming {
			status.Phase = kubeparkv1alpha1.SandboxPhaseResuming
		} else {
			status.Phase = kubeparkv1alpha1.SandboxPhaseProvisioning
		}
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionPodReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProvisioning, "creating sandbox pod")
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProvisioning, "creating sandbox pod")
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	status.PodName = pod.Name

	// A dead pod is deleted and recreated on the next pass — pod death is
	// not sandbox death.
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		if pod.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
		status.Phase = kubeparkv1alpha1.SandboxPhaseProvisioning
		status.PodIP = ""
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionPodReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProvisioning,
			fmt.Sprintf("pod %s, recreating", pod.Status.Phase))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if podReady(&pod) {
		if status.Phase != kubeparkv1alpha1.SandboxPhaseRunning {
			status.Phase = kubeparkv1alpha1.SandboxPhaseRunning
			// Start the idle clock even if no session ever opens
			// (R2-H-A); session closes move it forward later.
			if status.LastActivityTime == nil {
				now := metav1.Now()
				status.LastActivityTime = &now
			}
		}
		status.PodIP = pod.Status.PodIP
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionPodReady, metav1.ConditionTrue,
			kubeparkv1alpha1.ReasonRunning, "sandbox pod is ready")
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionReady, metav1.ConditionTrue,
			kubeparkv1alpha1.ReasonRunning, "sandbox is running")
	} else {
		if status.Phase != kubeparkv1alpha1.SandboxPhaseResuming {
			status.Phase = kubeparkv1alpha1.SandboxPhaseProvisioning
		}
		status.PodIP = ""
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionPodReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProvisioning, podPendingMessage(&pod))
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionReady, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonProvisioning, "waiting for sandbox pod")
	}

	// Template drift is surfaced, never acted on while running (R1-M2).
	if status.TemplateHash != "" && status.TemplateHash != currentHash {
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionTemplateOutdated, metav1.ConditionTrue,
			kubeparkv1alpha1.ReasonOutdated,
			"template changed; the new template applies on the next suspend/resume cycle")
	} else {
		r.setCondition(sb, status, kubeparkv1alpha1.ConditionTemplateOutdated, metav1.ConditionFalse,
			kubeparkv1alpha1.ReasonUpToDate, "pod matches the template")
	}
	return ctrl.Result{}, nil
}

// finalize cleans owned resources and applies the home retain policy.
func (r *SandboxReconciler) finalize(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(sb, SandboxFinalizer) {
		return ctrl.Result{}, nil
	}

	if sb.Status.Phase != kubeparkv1alpha1.SandboxPhaseTerminating {
		sb.Status.Phase = kubeparkv1alpha1.SandboxPhaseTerminating
		if err := r.Status().Update(ctx, sb); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Owned, namespace-local children. Owner references would collect these
	// eventually; deleting explicitly keeps teardown deterministic.
	for _, obj := range []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: sb.Namespace, Name: podspec.PodName(sb.Name)}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: sb.Namespace, Name: podspec.HostKeyName(sb.Name)}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: sb.Namespace, Name: podspec.NetPolName(sb.Name)}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: sb.Namespace, Name: saName(sb.Name)}},
	} {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// RoleBindings live in arbitrary grant namespaces; GC them by label.
	if err := r.gcRBAC(ctx, sb, nil); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.closeSessions(ctx, sb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyHomeRetainPolicy(ctx, sb); err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(sb, SandboxFinalizer)
	return ctrl.Result{}, r.Update(ctx, sb)
}

// closeSessions marks any open sessions Closed so the audit trail records
// why they ended.
func (r *SandboxReconciler) closeSessions(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) error {
	var sessions kubeparkv1alpha1.SandboxSessionList
	if err := r.List(ctx, &sessions, client.InNamespace(sb.Namespace)); err != nil {
		return err
	}
	now := metav1.Now()
	for i := range sessions.Items {
		s := &sessions.Items[i]
		if s.Spec.SandboxName != sb.Name || s.Status.State != kubeparkv1alpha1.SessionStateActive {
			continue
		}
		s.Status.State = kubeparkv1alpha1.SessionStateClosed
		s.Status.EndTime = &now
		s.Status.ExitReason = kubeparkv1alpha1.ExitReasonSandboxDeleted
		if err := r.Status().Update(ctx, s); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// applyHomeRetainPolicy deletes the home PVC only when the sandbox created
// it and asked for Delete; otherwise a kubepark-created PVC is labeled as
// an orphaned home. existingClaim PVCs are never touched.
func (r *SandboxReconciler) applyHomeRetainPolicy(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) error {
	if sb.Spec.Home != nil && sb.Spec.Home.ExistingClaim != "" {
		return nil
	}
	var pvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, types.NamespacedName{Namespace: sb.Namespace, Name: podspec.PVCName(sb.Name)}, &pvc)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if sb.Spec.Home != nil && sb.Spec.Home.RetainPolicy == kubeparkv1alpha1.RetainPolicyDelete {
		if err := r.Delete(ctx, &pvc); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	if pvc.Labels == nil {
		pvc.Labels = map[string]string{}
	}
	pvc.Labels[LabelOrphanedHome] = "true"
	return client.IgnoreNotFound(r.Update(ctx, &pvc))
}

func (r *SandboxReconciler) patchStatus(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, status *kubeparkv1alpha1.SandboxStatus) error {
	if equality(sb.Status, *status) {
		return nil
	}
	sb.Status = *status
	return client.IgnoreNotFound(r.Status().Update(ctx, sb))
}

func (r *SandboxReconciler) setCondition(sb *kubeparkv1alpha1.Sandbox, status *kubeparkv1alpha1.SandboxStatus, condType string, condStatus metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sb.Generation,
	})
}

func (r *SandboxReconciler) gatewayNamespace() string {
	if r.GatewayNamespace != "" {
		return r.GatewayNamespace
	}
	return OperatorNamespace()
}

func podReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func podPendingMessage(pod *corev1.Pod) string {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status != corev1.ConditionTrue {
			return fmt.Sprintf("pod not scheduled: %s", cond.Message)
		}
	}
	return "waiting for sandbox pod to become ready"
}

// SetupWithManager sets up the controller with the Manager.
func (r *SandboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kubeparkv1alpha1.Sandbox{},
		indexSandboxTemplate, func(obj client.Object) []string {
			return []string{obj.(*kubeparkv1alpha1.Sandbox).Spec.Template}
		}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kubeparkv1alpha1.Sandbox{},
		indexSandboxExistingClaim, func(obj client.Object) []string {
			sb := obj.(*kubeparkv1alpha1.Sandbox)
			if sb.Spec.Home == nil || sb.Spec.Home.ExistingClaim == "" {
				return nil
			}
			return []string{sb.Spec.Home.ExistingClaim}
		}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kubeparkv1alpha1.Sandbox{},
		indexSandboxAccessProfile, func(obj client.Object) []string {
			sb := obj.(*kubeparkv1alpha1.Sandbox)
			if sb.Spec.AccessProfile == "" {
				return nil
			}
			return []string{sb.Spec.AccessProfile}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeparkv1alpha1.Sandbox{}).
		Owns(&corev1.Pod{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&kubeparkv1alpha1.SandboxTemplate{},
			handler.EnqueueRequestsFromMapFunc(r.sandboxesForTemplate)).
		Watches(&kubeparkv1alpha1.AccessProfile{},
			handler.EnqueueRequestsFromMapFunc(r.sandboxesForAccessProfile)).
		Watches(&discoveryv1.EndpointSlice{},
			handler.EnqueueRequestsFromMapFunc(r.sandboxesForAPIServerEndpoints)).
		Named("sandbox").
		Complete(r)
}

// sandboxesForTemplate re-queues every sandbox referencing a changed
// template (drift detection, resume-time application).
func (r *SandboxReconciler) sandboxesForTemplate(ctx context.Context, obj client.Object) []ctrl.Request {
	var sandboxes kubeparkv1alpha1.SandboxList
	if err := r.List(ctx, &sandboxes,
		client.MatchingFields{indexSandboxTemplate: obj.GetName()}); err != nil {
		return nil
	}
	return toRequests(sandboxes.Items)
}

// sandboxesForAccessProfile re-queues every sandbox referencing a changed
// or deleted profile so its RBAC and RBACReady condition are re-evaluated.
func (r *SandboxReconciler) sandboxesForAccessProfile(ctx context.Context, obj client.Object) []ctrl.Request {
	var sandboxes kubeparkv1alpha1.SandboxList
	if err := r.List(ctx, &sandboxes,
		client.MatchingFields{indexSandboxAccessProfile: obj.GetName()}); err != nil {
		return nil
	}
	return toRequests(sandboxes.Items)
}

// sandboxesForAPIServerEndpoints re-renders every network policy when the
// API server addresses move.
func (r *SandboxReconciler) sandboxesForAPIServerEndpoints(ctx context.Context, obj client.Object) []ctrl.Request {
	if obj.GetNamespace() != metav1.NamespaceDefault ||
		obj.GetLabels()[discoveryv1.LabelServiceName] != "kubernetes" {
		return nil
	}
	var sandboxes kubeparkv1alpha1.SandboxList
	if err := r.List(ctx, &sandboxes); err != nil {
		return nil
	}
	return toRequests(sandboxes.Items)
}

func toRequests(items []kubeparkv1alpha1.Sandbox) []ctrl.Request {
	reqs := make([]ctrl.Request, 0, len(items))
	for i := range items {
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: items[i].Namespace, Name: items[i].Name,
		}})
	}
	return reqs
}

func valueOr(res *ctrl.Result) ctrl.Result {
	if res == nil {
		return ctrl.Result{}
	}
	return *res
}
