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

import "testing"

const sbName = "demo"
const nsAlice = "alice"

func TestParseSSHTarget(t *testing.T) {
	cases := []struct {
		in        string
		defaultNS string
		wantSB    string
		wantNS    string
		wantErr   bool
	}{
		{in: sbName + ".alice", wantSB: sbName, wantNS: nsAlice},
		{in: sbName, defaultNS: "team", wantSB: sbName, wantNS: "team"},
		{in: sbName, wantErr: true},
		{in: "", wantErr: true},
		{in: ".alice", wantErr: true},
		{in: sbName + ".", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ParseSSHTarget(tc.in, tc.defaultNS)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseSSHTarget(%q, %q): expected error", tc.in, tc.defaultNS)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSSHTarget(%q, %q): unexpected error %v", tc.in, tc.defaultNS, err)
			continue
		}
		if got.Sandbox != tc.wantSB || got.Namespace != tc.wantNS {
			t.Errorf("ParseSSHTarget(%q, %q) = %+v, want %s/%s", tc.in, tc.defaultNS, got, tc.wantNS, tc.wantSB)
		}
	}
}
