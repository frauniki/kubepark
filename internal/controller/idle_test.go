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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

func TestEffectiveIdleTimeout(t *testing.T) {
	tpl := &kubeparkv1alpha1.SandboxTemplate{
		Spec: kubeparkv1alpha1.SandboxTemplateSpec{
			DefaultIdleTimeout: &metav1.Duration{Duration: 5 * time.Minute},
		},
	}
	sbNoOverride := &kubeparkv1alpha1.Sandbox{}
	if got := effectiveIdleTimeout(sbNoOverride, tpl); got != 5*time.Minute {
		t.Errorf("expected template default 5m, got %v", got)
	}

	sbOverride := &kubeparkv1alpha1.Sandbox{
		Spec: kubeparkv1alpha1.SandboxSpec{IdleTimeout: &metav1.Duration{Duration: time.Minute}},
	}
	if got := effectiveIdleTimeout(sbOverride, tpl); got != time.Minute {
		t.Errorf("expected override 1m, got %v", got)
	}

	if got := effectiveIdleTimeout(sbNoOverride, &kubeparkv1alpha1.SandboxTemplate{}); got != 0 {
		t.Errorf("expected 0 (disabled) with no override or default, got %v", got)
	}
}

func TestIdleExpired(t *testing.T) {
	now := metav1.Now()
	old := metav1.NewTime(now.Add(-10 * time.Minute))

	if idleExpired(nil, time.Minute) {
		t.Error("nil lastActivity must not be considered expired")
	}
	if idleExpired(&old, 0) {
		t.Error("timeout 0 disables idle suspension")
	}
	if idleExpired(&now, time.Minute) {
		t.Error("recent activity must not be expired")
	}
	if !idleExpired(&old, time.Minute) {
		t.Error("stale activity must be expired")
	}
}
