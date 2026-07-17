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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

const (
	// defaultHeartbeatInterval is assumed when a session omits one.
	defaultHeartbeatInterval = 60 * time.Second
	// staleHeartbeatFactor closes an Active session whose last heartbeat is
	// older than factor * interval, so a gateway crash does not pin a
	// sandbox awake forever.
	staleHeartbeatFactor = 3
	// closedSessionRetention garbage-collects Closed sessions after this
	// long, keeping the audit trail bounded.
	closedSessionRetention = 168 * time.Hour
)

// SandboxSessionReconciler reaps stale Active sessions, garbage-collects old
// Closed sessions, and propagates a session's close time into the owning
// sandbox's idle clock.
type SandboxSessionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Now is overridable in tests; defaults to time.Now.
	Now func() time.Time
}

// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxsessions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxsessions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubepark.dev,resources=sandboxsessions/finalizers,verbs=update

// Reconcile drives one session through its lifecycle bookkeeping.
func (r *SandboxSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var session kubeparkv1alpha1.SandboxSession
	if err := r.Get(ctx, req.NamespacedName, &session); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch session.Status.State {
	case kubeparkv1alpha1.SessionStateActive:
		return r.reconcileActive(ctx, &session)
	case kubeparkv1alpha1.SessionStateClosed:
		return r.reconcileClosed(ctx, &session)
	default:
		// A freshly created session with no state yet: nothing to do until
		// the gateway marks it Active.
		log.V(1).Info("session has no state yet", "session", session.Name)
		return ctrl.Result{}, nil
	}
}

// reconcileActive reaps a session whose heartbeats have gone stale.
func (r *SandboxSessionReconciler) reconcileActive(ctx context.Context, session *kubeparkv1alpha1.SandboxSession) (ctrl.Result, error) {
	interval := defaultHeartbeatInterval
	if session.Spec.HeartbeatInterval != nil && session.Spec.HeartbeatInterval.Duration > 0 {
		interval = session.Spec.HeartbeatInterval.Duration
	}
	deadline := staleHeartbeatFactor * interval

	last := lastSeen(session)
	if last.IsZero() {
		// No activity stamp yet; check again after the deadline.
		return ctrl.Result{RequeueAfter: deadline}, nil
	}
	elapsed := r.now().Sub(last)
	if elapsed <= deadline {
		return ctrl.Result{RequeueAfter: deadline - elapsed}, nil
	}

	// Stale: close it, dating the end at the last heartbeat (not now) so a
	// gateway crash does not extend the sandbox's life by the deadline.
	end := metav1.NewTime(last)
	session.Status.State = kubeparkv1alpha1.SessionStateClosed
	session.Status.EndTime = &end
	session.Status.ExitReason = kubeparkv1alpha1.ExitReasonStaleHeartbeat
	if err := r.Status().Update(ctx, session); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

// reconcileClosed propagates the close time into the sandbox idle clock and
// garbage-collects the session once it ages out.
func (r *SandboxSessionReconciler) reconcileClosed(ctx context.Context, session *kubeparkv1alpha1.SandboxSession) (ctrl.Result, error) {
	if session.Status.EndTime != nil {
		if err := r.bumpSandboxActivity(ctx, session); err != nil {
			return ctrl.Result{}, err
		}
		age := r.now().Sub(session.Status.EndTime.Time)
		if age >= closedSessionRetention {
			return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, session))
		}
		return ctrl.Result{RequeueAfter: closedSessionRetention - age}, nil
	}
	return ctrl.Result{}, nil
}

// bumpSandboxActivity advances the sandbox's lastActivityTime to this
// session's end time (the idle clock only starts once sessions close).
func (r *SandboxSessionReconciler) bumpSandboxActivity(ctx context.Context, session *kubeparkv1alpha1.SandboxSession) error {
	var sb kubeparkv1alpha1.Sandbox
	err := r.Get(ctx, types.NamespacedName{Namespace: session.Namespace, Name: session.Spec.SandboxName}, &sb)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	end := session.Status.EndTime
	if sb.Status.LastActivityTime != nil && !end.After(sb.Status.LastActivityTime.Time) {
		return nil
	}
	patch := client.MergeFrom(sb.DeepCopy())
	sb.Status.LastActivityTime = end
	return client.IgnoreNotFound(r.Status().Patch(ctx, &sb, patch))
}

func (r *SandboxSessionReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// lastSeen returns the most recent activity timestamp of a session.
func lastSeen(session *kubeparkv1alpha1.SandboxSession) time.Time {
	if session.Status.LastActivityTime != nil {
		return session.Status.LastActivityTime.Time
	}
	if session.Status.StartTime != nil {
		return session.Status.StartTime.Time
	}
	return session.CreationTimestamp.Time
}

// SetupWithManager sets up the controller with the Manager.
func (r *SandboxSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeparkv1alpha1.SandboxSession{}).
		Named("sandboxsession").
		Complete(r)
}
