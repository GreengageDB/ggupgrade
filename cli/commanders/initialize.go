// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commanders

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/idl"
)

var execCommandHubStart = exec.Command
var execCommandHubCount = exec.Command

// CreateStateDir creates the state directory in the cli to ensure that at most
// one ggupgrade is occurring at the same time.
func CreateStateDir() (err error) {
	stateDir := utils.GetStateDir()

	err = os.Mkdir(stateDir, 0700)
	if os.IsExist(err) {
		log.Printf("State directory %s already present. Skipping.", stateDir)
		return nil
	}

	if err != nil {
		return xerrors.Errorf("creating state directory %q: %w", stateDir, err)
	}

	return nil
}

func StartHub(streams step.OutStreams) (err error) {
	running, err := IsHubRunning()
	if err != nil {
		return xerrors.Errorf("is hub running: %w", err)
	}

	if running {
		fmt.Fprint(streams.Stdout(), "Hub already running. Skipping.")
		return step.Skip
	}

	cmd := execCommandHubStart("ggupgrade", "hub", "--daemonize")
	log.Printf("Executing: %q", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("%q failed with %q: %w", cmd.String(), string(output), err)
	}

	_, err = streams.Stdout().Write(output)
	if err != nil {
		return err
	}

	return nil
}

func IsHubRunning() (bool, error) {
	script := `ps -ef | grep -wGc "[g]pupgrade hub"` // use square brackets to avoid finding yourself in matches
	_, err := execCommandHubCount("bash", "-c", script).Output()

	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ProcessState.ExitCode() == 1 { // hub not found
			return false, nil
		}
	}
	if err != nil { // grep failed
		return false, err
	}

	return true, nil
}

// nomerge: better naming
const err_message =
`Can not start migration because %v present in the cluster.

We don't support plpython2u in Greengage 7 beause Python 2 has been depricated for a while.
Please manually migrate every function that uses plpython2u to plpython3u.

After you done, execute the following query:

'''
%v
'''

Note that there steps should be done for each existing database.`


func CheckForObsoletePlpython(streams step.OutStreams, gphome string, port int, seedDir string) (err error) {
	// nomerge: do we really need to use this function in here?
	db, err := bootstrapConnectionFunc(idl.ClusterDestination_source, gphome, port, "template1")
	if err != nil {
		return err
	}

	// GetDatabases requires to specify script dirs, even though we don't generate them
	// in here. We can always refactor out this function, but it doesn't seem to be a problem.
	// 
	databases, err := GetDatabases(db, utils.System.DirFS(seedDir))

	err = db.Close()
	if err != nil {
		return err
	}

	for _, db_info := range databases {
		db, err := bootstrapConnectionFunc(idl.ClusterDestination_source, gphome, port, db_info.Datname)
		if err != nil {
			return err
		}

		plpythonu_count := -1
		row := db.QueryRow("SELECT COUNT(*) FROM pg_language WHERE lanname = 'plpythonu';")

		err = row.Scan(&plpythonu_count)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", db_info.Datname, err)
		}

		plpython2u_count := -1
		row = db.QueryRow("SELECT COUNT(*) FROM pg_language WHERE lanname = 'plpython2u';")	

		err = row.Scan(&plpython2u_count)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", db_info.Datname, err)
		}

		// Strictly speaking, we should check that both 
		// plpython2u_count and plpython2u_count are not equal 
		// to one instead of checking that they are not equal to zero,
		// as there is no way for pg_language view to have duplicating 
		// entries for them. But let's be conservative.

		// nomerge: actually report all the databases and all affected functions
	
		var found_languages string	
		var drop_command string

		if (plpythonu_count > 0 || plpython2u_count > 0) {
			if (plpythonu_count > 0 && plpython2u_count > 0) {
				found_languages = "plpython2u and plpythonu (an alias to plpython2u) are"
				drop_command = "DROP LANGUAGE plpython2u;\nDROP LANGUAGE plpythonu;"
			} else if (plpython2u_count > 0) {
				found_languages = "plpython2u is"
				drop_command = "DROP LANGUAGE plpython2u;"
			} else if (plpythonu_count > 0) {
				found_languages = "plpythonu (an alias to plpython2u) is"
				drop_command = "DROP LANGUAGE plpythonu;"
			} else {
				// no way to get here
				return xerrors.Errorf("internal error: unexpected condition")
			}

			formatted_err_message := fmt.Sprintf(err_message, found_languages, drop_command)
			return xerrors.Errorf("database '%v': %v", db_info.Datname, formatted_err_message)
		}
	}

	return nil
}
