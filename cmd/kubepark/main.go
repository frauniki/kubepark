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

// Command kubepark is the client CLI: it obtains short-lived SSH
// certificates (OIDC login or admin offline signing) and opens SSH
// connections to sandboxes through the kubepark gateway.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var errNotImplemented = errors.New("not implemented yet")

func main() {
	root := &cobra.Command{
		Use:           "kubepark",
		Short:         "Client CLI for kubepark sandboxes",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newLoginCommand(),
		newSSHCommand(),
		newAdminCommand(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Obtain a short-lived SSH certificate via OIDC",
		RunE:  func(_ *cobra.Command, _ []string) error { return errNotImplemented }, // M4
	}
}
