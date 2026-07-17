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
	"fmt"
	"strings"
)

// SSHTarget is a parsed SSH jump destination.
type SSHTarget struct {
	Sandbox   string
	Namespace string
}

// ParseSSHTarget parses the inner SSH destination host of the form
// "<sandbox>.<namespace>". Both components are DNS labels containing no
// dots, so the first dot is an unambiguous separator. A bare "<sandbox>"
// defaults the namespace to defaultNamespace.
func ParseSSHTarget(host, defaultNamespace string) (SSHTarget, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return SSHTarget{}, fmt.Errorf("empty ssh target")
	}
	sandbox, namespace, found := strings.Cut(host, ".")
	if !found {
		if defaultNamespace == "" {
			return SSHTarget{}, fmt.Errorf("target %q has no namespace and no default is set", host)
		}
		return SSHTarget{Sandbox: sandbox, Namespace: defaultNamespace}, nil
	}
	if sandbox == "" || namespace == "" {
		return SSHTarget{}, fmt.Errorf("invalid ssh target %q", host)
	}
	return SSHTarget{Sandbox: sandbox, Namespace: namespace}, nil
}
