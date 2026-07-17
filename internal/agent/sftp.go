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
	gliderssh "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

// sftpHandler serves the SFTP subsystem, which powers `sftp` and the modern
// scp protocol as well as IDE file sync. The client is already
// authenticated as the owner; the server exposes the pod filesystem with
// the process's own (non-root) permissions.
func sftpHandler(_ string) gliderssh.SubsystemHandler {
	return func(s gliderssh.Session) {
		server, err := sftp.NewServer(s)
		if err != nil {
			_ = s.Exit(1)
			return
		}
		defer func() { _ = server.Close() }()
		if err := server.Serve(); err != nil {
			// io.EOF is the normal end of an SFTP session.
			_ = s.Exit(0)
			return
		}
		_ = s.Exit(0)
	}
}
