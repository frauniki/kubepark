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
	"testing"
	"time"
)

func TestHeartbeatInterval(t *testing.T) {
	cases := []struct {
		idle time.Duration
		want time.Duration
	}{
		{idle: 0, want: 60 * time.Second},               // disabled -> floor
		{idle: 2 * time.Minute, want: 60 * time.Second}, // quarter (30s) < floor
		{idle: 8 * time.Minute, want: 2 * time.Minute},  // quarter (2m) > floor
		{idle: time.Hour, want: 15 * time.Minute},
	}
	for _, tc := range cases {
		if got := heartbeatInterval(tc.idle); got != tc.want {
			t.Errorf("heartbeatInterval(%v) = %v, want %v", tc.idle, got, tc.want)
		}
	}
}
