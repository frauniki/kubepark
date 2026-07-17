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

// HTTPTarget is a parsed HTTP routing host.
type HTTPTarget struct {
	Port      string
	Sandbox   string
	Namespace string
}

// maxLabelLen is the DNS single-label limit; the whole
// <port>--<sandbox>--<ns> must fit in one label so the assumed wildcard
// certificate (one level deep) covers it.
const maxLabelLen = 63

// ParseHTTPHost parses a routing host of the form
// "<port>--<sandbox>--<ns>.<baseDomain>". Parsing is LEFT-anchored: the
// first "--" ends the port, the second ends the sandbox, and the remainder
// is the namespace (which alone may legally contain "--"). The result is
// re-serialized and compared to the input label so an ambiguous or crafted
// host is rejected rather than mis-routed.
func ParseHTTPHost(host, baseDomain string) (HTTPTarget, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h // strip any port
	}

	label := host
	if baseDomain != "" {
		suffix := "." + strings.TrimPrefix(baseDomain, ".")
		if !strings.HasSuffix(host, suffix) {
			return HTTPTarget{}, fmt.Errorf("host %q is not under base domain %q", host, baseDomain)
		}
		label = strings.TrimSuffix(host, suffix)
	} else if h, _, ok := strings.Cut(host, "."); ok {
		label = h
	}

	if len(label) > maxLabelLen {
		return HTTPTarget{}, fmt.Errorf("routing label %q exceeds %d characters", label, maxLabelLen)
	}

	port, rest, ok := strings.Cut(label, "--")
	if !ok {
		return HTTPTarget{}, fmt.Errorf("host label %q missing separators", label)
	}
	sandbox, namespace, ok := strings.Cut(rest, "--")
	if !ok {
		return HTTPTarget{}, fmt.Errorf("host label %q missing namespace", label)
	}
	if port == "" || sandbox == "" || namespace == "" {
		return HTTPTarget{}, fmt.Errorf("host label %q has an empty component", label)
	}

	target := HTTPTarget{Port: port, Sandbox: sandbox, Namespace: namespace}
	// Round-trip check: the reconstructed label must equal the requested
	// one, so a sandbox name containing "--" (which CEL forbids) or any
	// other ambiguity cannot spoof another tenant's route.
	if reconstructed := target.Port + "--" + target.Sandbox + "--" + target.Namespace; reconstructed != label {
		return HTTPTarget{}, fmt.Errorf("host label %q is ambiguous", label)
	}
	return target, nil
}
