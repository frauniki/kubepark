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
	"strings"
	"testing"
)

func TestParseHTTPHost(t *testing.T) {
	base := "kubepark.example.com"
	cases := []struct {
		host    string
		wantP   string
		wantSB  string
		wantNS  string
		wantErr bool
	}{
		{host: "jupyter--demo--alice.kubepark.example.com", wantP: "jupyter", wantSB: sbName, wantNS: "alice"},
		// A namespace may legally contain "--"; left-anchoring handles it.
		{host: "web--demo--team--a.kubepark.example.com", wantP: "web", wantSB: sbName, wantNS: "team--a"},
		{host: "jupyter--demo--alice.other.com", wantErr: true},                        // wrong base domain
		{host: "demo.kubepark.example.com", wantErr: true},                             // missing separators
		{host: "jupyter--demo.kubepark.example.com", wantErr: true},                    // missing namespace
		{host: "--demo--alice.kubepark.example.com", wantErr: true},                    // empty port
		{host: strings.Repeat("a", 70) + "--x--y.kubepark.example.com", wantErr: true}, // too long
	}
	for _, tc := range cases {
		got, err := ParseHTTPHost(tc.host, base)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseHTTPHost(%q): expected error", tc.host)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseHTTPHost(%q): unexpected error %v", tc.host, err)
			continue
		}
		if got.Port != tc.wantP || got.Sandbox != tc.wantSB || got.Namespace != tc.wantNS {
			t.Errorf("ParseHTTPHost(%q) = %+v, want %s/%s/%s", tc.host, got, tc.wantP, tc.wantSB, tc.wantNS)
		}
	}
}
