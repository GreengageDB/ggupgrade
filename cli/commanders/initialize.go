// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commanders

import (
	"fmt"
	"log"
	"os"
	"bytes"
	"os/exec"
	"path/filepath"

	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/substeps"
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

We don't support plpython2u in Greengage 7 because Python 2 has been deprecated for a while.

Please manually migrate all functions using plpython2u to plpython3u.

Affected databases and functions are listed in this file:
'%v'

After this is done, execute the following query for each database:

'''
%v
'''
`


func CheckForObsoletePlpython(streams step.OutStreams, gphome string, port int, seedDir string) (err error) {
	version, err := greengage.Version(gphome)
	if err != nil {
		return err
	}

	if (version.Major != 6) {
		// This check is relevant only for 6.x.
		return nil;
	}

	db, err := bootstrapConnectionFunc(idl.ClusterDestination_source, gphome, port, "template1")
	if err != nil {
		return err
	}

	// GetDatabases requires to specify script dirs, even though we don't generate them
	// in here. We can always refactor this function, but it doesn't seem to be a problem right now.
	databases, err := GetDatabases(db, utils.System.DirFS(seedDir))

	err = db.Close()
	if err != nil {
		return err
	}

	var contents bytes.Buffer
	plpythonu_is_present := false
	plpython2u_is_present := false
	for _, db_info := range databases {
		db, err := bootstrapConnectionFunc(idl.ClusterDestination_source, gphome, port, db_info.Datname)
		if err != nil {
			return err
		}

		var plpythonu_count int
		row := db.QueryRow("SELECT COUNT(*) FROM pg_language WHERE lanname = 'plpythonu';")

		err = row.Scan(&plpythonu_count)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", db_info.Datname, err)
		}

		var plpython2u_count int
		row = db.QueryRow("SELECT COUNT(*) FROM pg_language WHERE lanname = 'plpython2u';")

		err = row.Scan(&plpython2u_count)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", db_info.Datname, err)
		}

		if plpython2u_count == 0 && plpythonu_count == 0 {
			continue
		}

		plpythonu_is_present  = plpythonu_is_present  || (plpythonu_count > 0)
		plpython2u_is_present = plpython2u_is_present || (plpython2u_count > 0)

		contents.WriteString(substeps.Divider);
		contents.WriteString("\n");
		contents.WriteString(fmt.Sprintf("Database: '%s'\n", db_info.Datname));
		contents.WriteString(substeps.Divider);
		contents.WriteString("\n");

		const query = "SELECT proname FROM pg_proc JOIN pg_language ON pg_proc.prolang = pg_language.oid WHERE pg_language.lanname = 'plpythonu' or pg_language.lanname = 'plpython2u';"
		rows, err := db.Query(query)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", db_info.Datname, err)
		}

		for rows.Next() {
			var proname string
			err = rows.Scan(&proname)
			if err != nil {
				return xerrors.Errorf("database '%v': %w", db_info.Datname, err)
			}

			contents.WriteString(proname);
			contents.WriteString("\n");
		}
	}

	if !plpythonu_is_present && !plpython2u_is_present {
		// no plpython2u in the cluster, can safely continue
		return nil
	}

	outputDir, err := utils.GetLogDir()
	if err != nil {
		return xerrors.Errorf("Internal error: unexpected condition")
	}

	const outputFile = "databases_with_plpython2u.pl"
	filePath := filepath.Join(outputDir, outputFile)
	err = utils.System.WriteFile(filePath, contents.Bytes(), 0644)
	if err != nil {
		return err
	}

	var found_languages string
	var drop_command string

	if plpythonu_is_present && plpython2u_is_present {
		found_languages = "plpython2u and plpythonu (an alias to plpython2u) are"
		drop_command = "DROP LANGUAGE IF EXISTS plpython2u;\nDROP LANGUAGE IF EXISTS plpythonu;"
	} else if plpython2u_is_present {
		found_languages = "plpython2u is"
		drop_command = "DROP LANGUAGE IF EXISTS plpython2u;"
	} else if plpythonu_is_present {
		found_languages = "plpythonu (an alias to plpython2u) is"
		drop_command = "DROP LANGUAGE IF EXISTS plpythonu;"
	} else {
		// no way to get here
		return xerrors.Errorf("Internal error: unexpected condition")
	}

	return xerrors.Errorf(err_message, found_languages, filePath, drop_command)
}
