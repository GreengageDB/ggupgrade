// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/testutils/exectest"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
)

func CheckEnvironment(agentHostsIncludingCoordinator []string, sourceGphome string, intermediateGphome string) error {
	errs := make(chan error, len(agentHostsIncludingCoordinator))
	var wg sync.WaitGroup

	for _, host := range agentHostsIncludingCoordinator {
		wg.Add(1)
		go func(host string, sourceGphome string, targetGphome string) {
			defer wg.Done()

			errs <- CheckEnvironmentOnSegment(host, sourceGphome, targetGphome)
		}(host, sourceGphome, intermediateGphome)
	}

	wg.Wait()
	close(errs)

	var err error
	for e := range errs {
		err = errorlist.Append(err, e)
	}

	if err != nil {
		nextAction := `On all segments remove sourcing greengage_path.sh and setting any Greengage variables
in .bashrc or .bash_profile. In a fresh shell re-run ggupgrade.`
		return utils.NewNextActionErr(err, nextAction)
	}

	return nil
}

var pathCommand = exec.Command
var ldLibraryPathCommand = exec.Command

// XXX: for internal testing only
func SetPathCommand(command exectest.Command) {
	pathCommand = command
}

func SetLdLibraryPathCommand(command exectest.Command) {
	ldLibraryPathCommand = command
}

// XXX: for internal testing only
func ResetPathCommand() {
	pathCommand = exec.Command
}

func ResetLdLibraryPathCommand() {
	ldLibraryPathCommand = exec.Command
}

// CheckEnvironmentOnSegment ensures that multiple versions of Greengage
// environments are not mixed. Use ssh instead of gRPC since our utilities like
// gpinitsystem, gpstart, gpstop, etc. use ssh internally. This checks up front
// for the following error as described here:
// https://web.archive.org/web/20220506055918/https://groups.google.com/a/greenplum.org/g/gpdb-dev/c/JN-YwjCCReY/m/0L9wBOvlAQAJ
func CheckEnvironmentOnSegment(host string, sourceGphome string, targetGphome string) error {
	// check $PATH
	cmd := pathCommand("ssh", "-q", host, "echo", "$PATH")
	log.Printf("Executing: %q", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("%q failed with %q: %w", cmd.String(), string(output), err)
	}

	log.Printf("Output: %q", output)
	path := string(output)
	if strings.Contains(path, sourceGphome) || strings.Contains(path, targetGphome) {
		return fmt.Errorf("on host %s PATH contains GPHOME", host)
	}

	// check $LD_LIBRARY_PATH
	cmd = ldLibraryPathCommand("ssh", "-q", host, "echo", "$LD_LIBRARY_PATH")
	log.Printf("Executing: %q", cmd.String())
	output, err = cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("%q failed with %q: %w", cmd.String(), string(output), err)
	}

	log.Printf("Output: %q", output)
	ldLibraryPath := string(output)
	if strings.Contains(ldLibraryPath, sourceGphome) || strings.Contains(ldLibraryPath, targetGphome) {
		return fmt.Errorf("on host %s LD_LIBRARY_PATH contains GPHOME", host)
	}

	return nil
}
