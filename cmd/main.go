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
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// agentBinaryName is the file name "agent install" writes and the argv[0]
// basename that selects agent mode when the copy is executed directly.
const agentBinaryName = "agent"

// main dispatches between the modes of the single kubepark server binary:
// "operator" runs the controller manager, "gateway" runs the SSH/HTTP
// gateway, and "agent" runs the in-sandbox SSH agent. Sandbox init
// containers copy this binary into the pod via "agent install"; the copy is
// named "agent", so an argv[0] basename of "agent" also selects agent mode.
func main() {
	if filepath.Base(os.Args[0]) == agentBinaryName {
		os.Args = append([]string{os.Args[0], agentBinaryName}, os.Args[1:]...)
	}

	root := &cobra.Command{
		Use:           "kubepark",
		Short:         "kubepark server binary (operator | gateway | agent)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newOperatorCommand(), newGatewayCommand(), newAgentCommand())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
