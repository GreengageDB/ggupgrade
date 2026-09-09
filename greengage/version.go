// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package greengage

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/blang/semver/v4"
	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/testutils/exectest"
)

var versionCommand = exec.Command

// XXX: for internal testing only
func SetVersionCommand(command exectest.Command) {
	versionCommand = command
}

// XXX: for internal testing only
func ResetVersionCommand() {
	versionCommand = exec.Command
}

func Version(gphome string) (semver.Version, error) {
	cmd := versionCommand(filepath.Join(gphome, "bin", "postgres"), "--gp-version")
	cmd.Env = []string{}

	log.Printf("Executing: %q", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return semver.Version{}, fmt.Errorf("%q failed with %q: %w", cmd.String(), string(output), err)
	}

	pattern := regexp.MustCompile(`postgres \(Green(?:gage|plum) Database\) (\d+\.\d+\.\d+)`)
	rawVersion := string(output)
	matches := pattern.FindStringSubmatch(rawVersion)

	if len(matches) < 2 {
		return semver.Version{}, xerrors.Errorf(`Greengage version %q is not of the form "postgres (Green(gage|plum) Database) #.#.#"`, rawVersion)
	}

	version, err := semver.Parse(matches[1])
	if err != nil {
		return semver.Version{}, xerrors.Errorf("parsing Greengage version %q: %w", rawVersion, err)
	}

	return version, nil
}
