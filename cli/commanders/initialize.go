// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commanders

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/xerrors"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/substeps"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
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
	script := `ps -ef | grep -wGc "[g]gupgrade hub"` // use square brackets to avoid finding yourself in matches
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

const OutputFilePlpython = "databases_with_plpython2u.txt"

const ErrorMessagePlpython = `Can not start migration because %v present in the cluster.

We don't support plpython2u in Greengage 7 because Python 2 has been deprecated for a while.

Please manually migrate all functions using plpython2u to plpython3u.

Affected databases and functions are listed in this file:
'%v'

After this is done, execute the following query for each affected database:

'''
%v
'''
`

func CheckForObsoletePlpython(streams step.OutStreams, gphome string, port int, seedDir string) (err error) {
	version, err := greengage.Version(gphome)
	if err != nil {
		return err
	}

	if version.Major != 6 {
		// This check is relevant only for 6.x.
		return nil
	}

	db, err := bootstrapConnectionFunc(idl.ClusterDestination_source, gphome, port, "template1")
	if err != nil {
		return err
	}

	databases, err := GetDatabases(db)

	// Whether the above call succeeds or not, close the database first
	dbErr := db.Close()
	if dbErr != nil {
		return dbErr
	}
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

		defer func() {
			if cErr := db.Close(); cErr != nil {
				err = errorlist.Append(err, cErr)
			}
		}()

		const languageQuery = "SELECT EXISTS(SELECT * FROM pg_catalog.pg_language WHERE lanname = 'plpythonu') plpythonu, EXISTS(SELECT * FROM pg_catalog.pg_language WHERE lanname = 'plpython2u') plpython2u;"

		var plpythonuIsPresentInDatabase bool
		var plpython2uIsPresentInDatabase bool
		row := db.QueryRow(languageQuery)
		err = row.Scan(&plpythonuIsPresentInDatabase, &plpython2uIsPresentInDatabase)
		if err != nil {
			return xerrors.Errorf("database %v: %w", dbInfo.QuotedDatname, err)
		}

		if !plpython2uIsPresentInDatabase && !plpythonuIsPresentInDatabase {
			continue
		}

		plpythonuIsPresent = plpythonuIsPresent || plpythonuIsPresentInDatabase
		plpython2uIsPresent = plpython2uIsPresent || plpython2uIsPresentInDatabase

		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")
		contents.WriteString(fmt.Sprintf("Database: %s\n", dbInfo.QuotedDatname))
		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")

		const functionQuery = `
SELECT quote_ident(c.proname) proname, pg_catalog.pg_get_function_arguments(c.oid) args, quote_ident(n.nspname) nspname
    FROM pg_catalog.pg_proc c
    JOIN pg_catalog.pg_language l ON c.prolang = l.oid
    JOIN pg_catalog.pg_namespace n ON c.pronamespace = n.oid
    WHERE l.lanname in ('plpythonu', 'plpython2u');
`
		rows, err := db.Query(functionQuery)
		if err != nil {
			return xerrors.Errorf("database %v: %w", dbInfo.QuotedDatname, err)
		}
		defer func() {
			if cErr := rows.Close(); cErr != nil {
				err = errorlist.Append(err, cErr)
			}
		}()

		for rows.Next() {
			var proname string
			var args string
			var nspname string
			err = rows.Scan(&proname, &args, &nspname)
			if err != nil {
				return xerrors.Errorf("database %v: %w", dbInfo.QuotedDatname, err)
			}

			functionSignature := fmt.Sprintf("%s.%s(%s)\n", nspname, proname, args)
			contents.WriteString(functionSignature)
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

	filePath := filepath.Join(outputDir, OutputFilePlpython)
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
