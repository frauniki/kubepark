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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newAgentCommand runs the in-sandbox SSH agent. The "install" subcommand is
// used by the sandbox init container to copy this binary into a volume
// shared with the user container (the image is distroless, so there is no
// cp). The copy is named "agent" so that executing it directly re-enters
// agent mode via the argv[0] dispatch in main.
func newAgentCommand() *cobra.Command {
	agent := &cobra.Command{
		Use:   agentBinaryName,
		Short: "Run the in-sandbox SSH agent",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Implemented in milestone M3.
			return errors.New("the agent is not implemented yet")
		},
	}
	agent.AddCommand(&cobra.Command{
		Use:   "install <dir>",
		Short: "Copy this binary into <dir>/agent (init container helper)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return installSelf(args[0])
		},
	})
	return agent
}

// installSelf copies the currently running executable to <dir>/agent,
// writing to a temporary file first so the final rename is atomic.
func installSelf(dir string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve own executable: %w", err)
	}
	src, err := os.Open(self)
	if err != nil {
		return fmt.Errorf("open %s: %w", self, err)
	}
	defer func() { _ = src.Close() }()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".agent-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	dst := filepath.Join(dir, agentBinaryName)
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}
