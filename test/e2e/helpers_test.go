//go:build e2e

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

package e2e

import (
	"os"
	"os/exec"
	"strings"
)

// command builds an *exec.Cmd rooted at the project directory so relative
// paths (charts/, cmd/) resolve.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	if dir := projectDir(); dir != "" {
		cmd.Dir = dir
	}
	return cmd
}

// projectDir walks up from the test package to the module root.
func projectDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// test/e2e -> repo root
	return strings.TrimSuffix(wd, "/test/e2e")
}

// loadImage loads a local image into the Kind cluster named by KIND_CLUSTER
// (default kubepark-test-e2e).
func loadImage(img string) error {
	cluster := os.Getenv("KIND_CLUSTER")
	if cluster == "" {
		cluster = "kubepark-test-e2e"
	}
	kind := os.Getenv("KIND")
	if kind == "" {
		kind = "kind"
	}
	cmd := command(kind, "load", "docker-image", img, "--name", cluster)
	return cmd.Run()
}

// imageRepo/imageTag split "repo:tag" (repo may itself contain a registry
// port, so split on the last colon).
func imageRepo(img string) string {
	if i := strings.LastIndex(img, ":"); i >= 0 && !strings.Contains(img[i:], "/") {
		return img[:i]
	}
	return img
}

func imageTag(img string) string {
	if i := strings.LastIndex(img, ":"); i >= 0 && !strings.Contains(img[i:], "/") {
		return img[i+1:]
	}
	return "latest"
}
