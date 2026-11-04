// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/utils"
)

func GetCoordinatorDynamicLibraryPath(gpconfigOutput string, version semver.Version) (string, error) {
	var prefix string
	switch version.Major {
	case 5, 6:
		prefix = "Master  value:"
	case 7:
		prefix = "Coordinator value:"
	}

	value := ""
	scanner := bufio.NewScanner(strings.NewReader(gpconfigOutput))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			line = strings.TrimPrefix(line, prefix)
			value = strings.TrimSpace(line)
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", xerrors.Errorf("scanning gpconfig dynamic_library_path output: %w", err)
	}

	return value, nil
}

func AppendDynamicLibraryPath(intermediate *greengage.Cluster, toAppend string) error {
	stream := &step.BufferedStreams{}

	// get current dynamic_library_path from the intermediate target cluster
	err := intermediate.RunGreengageCmdWithEnvironment(stream,
		"gpconfig", []string{"-s", "dynamic_library_path"},
		utils.FilterEnv([]string{"USER"})) // gpconfig requires the USER environment variable
	if err != nil {
		return err
	}

	currentValue, err := GetCoordinatorDynamicLibraryPath(stream.StdoutBuf.String(), intermediate.Version)
	if err != nil {
		return err
	}

	if currentValue == "" {
		fmt.Errorf(`Target cluster is missing value for dynamic_library_path. Expected "$libdir", but found %q: %w`, stream.StdoutBuf.String(), err)
	}

	// append the user specified dynamic_library_path to the current value and remove any duplicates
	dynamicLibraryPath := utils.RemoveDuplicates(append(
		strings.Split(currentValue, ":"),
		strings.Split(toAppend, ":")...))

	// set the dynamic_library_path
	err = intermediate.RunGreengageCmdWithEnvironment(stream,
		"gpconfig",
		[]string{"-c", "dynamic_library_path", "-v", strings.Join(dynamicLibraryPath, ":")},
		utils.FilterEnv([]string{"USER"})) // gpconfig requires the USER environment variable
	if err != nil {
		return err
	}

	return intermediate.RunGreengageCmd(stream, "gpstop", "-u")
}
