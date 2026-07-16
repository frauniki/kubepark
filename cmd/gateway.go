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

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// newGatewayCommand runs the kubepark gateway: an SSH jump host that
// authenticates clients with CA-signed certificates and routes direct-tcpip
// channels to sandbox pods, plus an HTTP reverse proxy for exposed ports.
func newGatewayCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gateway",
		Short: "Run the kubepark SSH/HTTP gateway",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Implemented in milestone M3 (SSH) and M5 (HTTP).
			return errors.New("the gateway is not implemented yet")
		},
	}
}
