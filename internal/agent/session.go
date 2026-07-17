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

package agent

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	gliderssh "github.com/gliderlabs/ssh"
)

// sessionManager provides tmux-style continuity: interactive PTY sessions
// attach to a single long-lived shell that survives client disconnects
// (though not pod death — the honest boundary). Non-interactive exec (scp,
// rsync, `ssh host cmd`) runs as an ephemeral child instead.
type sessionManager struct {
	cfg Config

	mu       sync.Mutex
	ptmx     *os.File
	shellCmd *exec.Cmd
	attached int
}

func newSessionManager(cfg Config) *sessionManager {
	return &sessionManager{cfg: cfg}
}

// handle dispatches a session to exec or the persistent shell.
func (m *sessionManager) handle(s gliderssh.Session) {
	if len(s.Command()) > 0 {
		m.runExec(s)
		return
	}
	ptyReq, winCh, isPty := s.Pty()
	if !isPty {
		// No PTY and no command: run a login shell reading stdin to EOF.
		m.runExec(s)
		return
	}
	m.attachShell(s, ptyReq, winCh)
}

// runExec runs a one-off command (or a non-interactive shell) as a child
// process, wiring stdio directly. This is the scp/rsync/`ssh host cmd`
// path.
func (m *sessionManager) runExec(s gliderssh.Session) {
	name, args := m.shellInvocation(s.Command())
	cmd := exec.Command(name, args...)
	cmd.Dir = m.cfg.HomeDir
	cmd.Env = append(os.Environ(), "HOME="+m.cfg.HomeDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = s.Exit(1)
		return
	}
	cmd.Stdout = s
	cmd.Stderr = s.Stderr()
	if err := cmd.Start(); err != nil {
		_ = s.Exit(127)
		return
	}
	go func() {
		_, _ = io.Copy(stdin, s)
		_ = stdin.Close()
	}()
	_ = s.Exit(waitStatus(cmd.Wait()))
}

// attachShell attaches the client to the shared persistent PTY, starting it
// on first use.
func (m *sessionManager) attachShell(s gliderssh.Session, ptyReq gliderssh.Pty, winCh <-chan gliderssh.Window) {
	ptmx, err := m.ensureShell(ptyReq.Term)
	if err != nil {
		_ = s.Exit(1)
		return
	}

	m.mu.Lock()
	m.attached++
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.attached--
		m.mu.Unlock()
	}()

	go func() {
		for win := range winCh {
			_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)})
		}
	}()

	// Bridge the client and the shared PTY. When the client disconnects the
	// copies end but the shell keeps running for the next attach.
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(ptmx, s); done <- struct{}{} }()
	go func() { _, _ = io.Copy(s, ptmx); done <- struct{}{} }()
	<-done
}

// ensureShell starts the persistent shell under a PTY if it is not already
// running, and returns the PTY master.
func (m *sessionManager) ensureShell(term string) (*os.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ptmx != nil && m.shellCmd != nil && m.shellCmd.ProcessState == nil {
		return m.ptmx, nil
	}

	name, args := m.shellInvocation(nil)
	cmd := exec.Command(name, args...)
	cmd.Dir = m.cfg.HomeDir
	if term == "" {
		term = "xterm-256color"
	}
	cmd.Env = append(os.Environ(), "HOME="+m.cfg.HomeDir, "TERM="+term)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	m.ptmx = ptmx
	m.shellCmd = cmd
	// Reap the shell so a persistent session that finally exits does not
	// leave a zombie; drop the shared PTY so the next attach restarts it.
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.shellCmd == cmd {
			_ = m.ptmx.Close()
			m.ptmx = nil
			m.shellCmd = nil
		}
		m.mu.Unlock()
	}()
	return ptmx, nil
}

// shellInvocation resolves the command to run for a session. A
// client-supplied command runs through the shell (scp, rsync, `ssh host
// cmd`); an interactive session always gets a login shell. The template
// command is the pod's main workload, not the interactive shell, so it is
// never used here — it is supervised separately (see startMainProcess).
func (m *sessionManager) shellInvocation(clientCmd []string) (string, []string) {
	shell := loginShell()
	if len(clientCmd) > 0 {
		return shell, []string{"-c", joinCommand(clientCmd)}
	}
	return shell, []string{"-l"}
}

// startMainProcess launches the template command (if any) as a supervised
// background child: the pod's long-running workload, independent of any SSH
// session. It is best-effort — if the workload exits, SSH access remains.
func (m *sessionManager) startMainProcess() {
	if len(m.cfg.Command) == 0 {
		return
	}
	cmd := exec.Command(m.cfg.Command[0], m.cfg.Command[1:]...)
	cmd.Dir = m.cfg.HomeDir
	cmd.Env = append(os.Environ(), "HOME="+m.cfg.HomeDir)
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}

func loginShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	for _, candidate := range []string{"/bin/bash", "/bin/sh"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "/bin/sh"
}

// joinCommand renders an SSH command vector the way OpenSSH does: the client
// already shell-quoted it into a single string per element boundary, so a
// space join reproduces the original command line.
func joinCommand(cmd []string) string {
	return strings.Join(cmd, " ")
}

func waitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if e, ok := err.(*exec.ExitError); ok {
		exitErr = e
	}
	if exitErr != nil {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus()
		}
	}
	return 1
}
