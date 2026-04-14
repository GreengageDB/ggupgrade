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

const ErrorMessagePlpython =
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
	plpythonuIsPresent := false
	plpython2uIsPresent := false
	for _, dbInfo := range databases {
		db, err := bootstrapConnectionFunc(idl.ClusterDestination_source, gphome, port, dbInfo.Datname)
		if err != nil {
			return err
		}

		var plpythonuCount int
		row := db.QueryRow("SELECT COUNT(*) FROM pg_language WHERE lanname = 'plpythonu';")

		err = row.Scan(&plpythonuCount)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", dbInfo.Datname, err)
		}

		var plpython2uCount int
		row = db.QueryRow("SELECT COUNT(*) FROM pg_language WHERE lanname = 'plpython2u';")

		err = row.Scan(&plpython2uCount)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", dbInfo.Datname, err)
		}

		if plpython2uCount == 0 && plpythonuCount == 0 {
			continue
		}

		plpythonuIsPresent  = plpythonuIsPresent  || (plpythonuCount > 0)
		plpython2uIsPresent = plpython2uIsPresent || (plpython2uCount > 0)

		contents.WriteString(substeps.Divider);
		contents.WriteString("\n");
		contents.WriteString(fmt.Sprintf("Database: '%s'\n", dbInfo.Datname));
		contents.WriteString(substeps.Divider);
		contents.WriteString("\n");

		const query = "SELECT proname FROM pg_proc JOIN pg_language ON pg_proc.prolang = pg_language.oid WHERE pg_language.lanname = 'plpythonu' or pg_language.lanname = 'plpython2u';"
		rows, err := db.Query(query)
		if err != nil {
			return xerrors.Errorf("database '%v': %w", dbInfo.Datname, err)
		}

		for rows.Next() {
			var proname string
			err = rows.Scan(&proname)
			if err != nil {
				return xerrors.Errorf("database '%v': %w", dbInfo.Datname, err)
			}

			contents.WriteString(proname);
			contents.WriteString("\n");
		}
	}

	if !plpythonuIsPresent && !plpython2uIsPresent {
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

	var foundLanguages string
	var dropCommand string

	if plpythonuIsPresent && plpython2uIsPresent {
		foundLanguages = "plpython2u and plpythonu (an alias to plpython2u) are"
		dropCommand = "DROP LANGUAGE IF EXISTS plpython2u;\nDROP LANGUAGE IF EXISTS plpythonu;"
	} else if plpython2uIsPresent {
		foundLanguages = "plpython2u is"
		dropCommand = "DROP LANGUAGE IF EXISTS plpython2u;"
	} else if plpythonuIsPresent {
		foundLanguages = "plpythonu (an alias to plpython2u) is"
		dropCommand = "DROP LANGUAGE IF EXISTS plpythonu;"
	} else {
		// no way to get here
		return xerrors.Errorf("Internal error: unexpected condition")
	}

	return xerrors.Errorf(ErrorMessagePlpython, foundLanguages, filePath, dropCommand)
}
