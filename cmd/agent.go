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
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/frauniki/kubepark/internal/agent"
)

// newAgentCommand runs the in-sandbox SSH agent. The "install" subcommand is
// used by the sandbox init container to copy this binary into a volume
// shared with the user container (the image is distroless, so there is no
// cp). The copy is named "agent" so that executing it directly re-enters
// agent mode via the argv[0] dispatch in main. Everything after "--" is the
// template's long-running command.
func newAgentCommand() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   agentBinaryName,
		Short: "Run the in-sandbox SSH agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := agent.ConfigFromEnv(argsAfterDashDash(cmd, args))
			if err != nil {
				return err
			}
			server, err := agent.NewServer(cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "kubepark-agent listening on %s for owner %s\n", cfg.Addr, cfg.Owner)
			return server.ListenAndServe()
		},
	}
	// Pass the template command through verbatim after "--".
	agentCmd.Flags().SetInterspersed(false)
	agentCmd.AddCommand(&cobra.Command{
		Use:   "install <dir>",
		Short: "Copy this binary into <dir>/agent (init container helper)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return installSelf(args[0])
		},
	})
	return agentCmd
}

// argsAfterDashDash returns the positional args that followed "--" on the
// command line (the template command), or all args if no "--" was present.
func argsAfterDashDash(cmd *cobra.Command, args []string) []string {
	if idx := cmd.ArgsLenAtDash(); idx >= 0 {
		return args[idx:]
	}
	return args
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
