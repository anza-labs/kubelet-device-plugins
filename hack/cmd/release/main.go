// Copyright 2024 anza-labs contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	semver "github.com/Masterminds/semver/v3"

	"sigs.k8s.io/yaml"
)

const (
	defaultKvmPluginImageName = "kvm"
	defaultKvmPluginImageRef  = "ghcr.io/anza-labs/kvm-device-plugin"
	defaultTunPluginImageName = "tun"
	defaultTunPluginImageRef  = "ghcr.io/anza-labs/tun-device-plugin"
)

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitCmd(args ...string) error {
	return runCommand("git", args...)
}

func branchPrep(version, fullVersion string) error {
	branch := fmt.Sprintf("release-%s", version)

	if branchExists(branch) {
		if err := switchToBranch(branch); err != nil {
			return err
		}
	} else {
		if err := createBranch(branch); err != nil {
			return err
		}
	}

	return gitCmd(
		"merge",
		"origin/main",
		"-m", fmt.Sprintf("chore(%s): merge changes for %s", version, fullVersion),
		"--signoff",
	)
}

func branchExists(branchName string) bool {
	cmd := exec.Command("git", "branch", "--list", branchName)
	output, _ := cmd.Output()
	return strings.Contains(string(output), branchName)
}

func switchToBranch(branchName string) error {
	return gitCmd("checkout", branchName)
}

func createBranch(branchName string) error {
	return gitCmd("checkout", "-b", branchName)
}

func parseVersion(version string) (string, error) {
	parsed, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	return fmt.Sprintf("%d.%d", parsed.Major(), parsed.Minor()), nil
}

func createKustomization(resources []string, images []map[string]string) map[string]interface{} {
	return map[string]interface{}{
		"namespace":  "anza-labs-kubelet-plugins",
		"namePrefix": "kubelet-",
		"resources":  resources,
		"images":     images,
	}
}

func writeKustomization(kustomization map[string]interface{}, filepath string) error {
	content, err := yaml.Marshal(kustomization)
	if err != nil {
		return fmt.Errorf("failed to marshal kustomization: %w", err)
	}

	return os.WriteFile(filepath, content, 0644)
}

func release(version, fullVersion string) error {
	if err := gitCmd("add", "."); err != nil {
		return err
	}
	if err := gitCmd(
		"commit",
		"-sm", fmt.Sprintf("chore(%s): create release commit %s", version, fullVersion),
	); err != nil {
		return err
	}
	if err := gitCmd("push", "origin", fmt.Sprintf("release-%s", version)); err != nil {
		return err
	}
	if err := gitCmd("tag", fullVersion); err != nil {
		return err
	}
	return gitCmd("push", "--tags")
}

func main() {
	versionFlag := flag.String("version", "", "Tagged version to build")
	kvmImageFlag := flag.String("kvm-plugin-image-name", defaultKvmPluginImageName, "Default image name")
	newKvmImageFlag := flag.String("kvm-plugin-image", defaultKvmPluginImageRef, "Default image reference")
	tunImageFlag := flag.String("tun-plugin-image-name", defaultTunPluginImageName, "Default image name")
	newTunImageFlag := flag.String("tun-plugin-image", defaultTunPluginImageRef, "Default image reference")

	flag.Parse()

	resources := []string{"./config/plugin", "./config/rbac"}

	version, err := parseVersion(*versionFlag)
	if err != nil {
		log.Fatalf("Failed to parse version: %v", err)
	}

	if err := branchPrep(version, *versionFlag); err != nil {
		log.Fatalf("Failed to prepare branch: %v", err)
	}

	kustomization := createKustomization(resources, []map[string]string{
		{
			"name":    *kvmImageFlag,
			"newName": *newKvmImageFlag,
			"newTag":  *versionFlag,
		},
		{
			"name":    *tunImageFlag,
			"newName": *newTunImageFlag,
			"newTag":  *versionFlag,
		},
	})
	if err := writeKustomization(kustomization, "./kustomization.yaml"); err != nil {
		log.Fatalf("Failed to write kustomization: %v", err)
	}

	if err := release(version, *versionFlag); err != nil {
		log.Fatalf("Failed to release: %v", err)
	}
}
