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
		_, err = fmt.Fprint(streams.Stdout(), "Hub already running. Skipping.")
		if err != nil {
			return err
		}
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
		if exitError.ExitCode() == 1 { // hub not found
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

const OutputFileUnsupportedUniqueIndexes = "partitioned_tables_with_unsupported_unique_indexes.txt"

const ErrorMessageUnsupportedUniqueIndexes = `Can not start migration because the cluster has unique indexes on partitioned tables that do not contain all of the partitioning columns.

Greengage 7 requires a unique index on a partitioned table to contain every partitioning column:

'''
ERROR:  unique constraint on partitioned table must include all partitioning columns
DETAIL:  UNIQUE constraint on table "sales" lacks column "office_id" which is part of the partition key.
'''

Such an index has to be dropped before the upgrade and cannot be recreated afterwards. Were the
migration to start, the index would be lost and the finalize phase would fail on a cluster that has
already been upgraded.

Affected databases, tables and indexes are listed in this file:
'%v'

For each of them either drop the index:

'''
DROP INDEX <schema>.<index>;
'''

or recreate it with the partitioning columns included:

'''
DROP INDEX <schema>.<index>;
CREATE UNIQUE INDEX <index> ON <schema>.<table> (<original columns>, <missing columns>);
'''
`

// unsupportedUniqueIndexesQuery lists the unique indexes on partitioned tables that Greengage 7
// does not allow, together with the partitioning columns they are missing. Indexes on the child
// partitions are not reported separately as pg_partition only holds the root of a hierarchy, and
// unique constraints need no reporting at all since Greengage 6 already refuses to create one that
// does not contain the partitioning columns.
const unsupportedUniqueIndexesQuery = `
WITH partition_keys AS
(
   SELECT DISTINCT
      p.parrelid AS relid,
      key.attnum
   FROM
      pg_catalog.pg_partition p,
      unnest(p.paratts) AS key(attnum)
)
SELECT
   pg_catalog.quote_ident(n.nspname) AS schemaname,
   pg_catalog.quote_ident(c.relname) AS tablename,
   pg_catalog.quote_ident(i.relname) AS indexname,
   pg_catalog.string_agg(pg_catalog.quote_ident(a.attname), ', ' ORDER BY a.attnum) AS missing_columns
FROM
   pg_catalog.pg_index x
   JOIN pg_catalog.pg_class i ON i.oid = x.indexrelid
   JOIN pg_catalog.pg_class c ON c.oid = x.indrelid
   JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   JOIN partition_keys k ON k.relid = x.indrelid
   JOIN pg_catalog.pg_attribute a ON a.attrelid = x.indrelid AND a.attnum = k.attnum
WHERE
   x.indisunique
   AND k.attnum <> ALL (x.indkey::pg_catalog.int2[])
GROUP BY 1, 2, 3
ORDER BY 1, 2, 3;
`

// CheckForUnsupportedUniqueIndexes stops the upgrade of a 6.x cluster that has a unique index on a
// partitioned table which does not contain all of the partitioning columns. The initialize data
// migration scripts drop such an index and the finalize ones replay its definition, which Greengage
// 7 rejects. Nothing can recreate it there, so the operator has to decide what happens to it before
// anything is dropped.
func CheckForUnsupportedUniqueIndexes(streams step.OutStreams, gphome string, port int) (err error) {
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
	indexesArePresent := false
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

		rows, err := db.Query(unsupportedUniqueIndexesQuery)
		if err != nil {
			return xerrors.Errorf("database %v: %w", dbInfo.QuotedDatname, err)
		}
		defer func() {
			if cErr := rows.Close(); cErr != nil {
				err = errorlist.Append(err, cErr)
			}
		}()

		var databaseContents bytes.Buffer
		for rows.Next() {
			var nspname string
			var relname string
			var indexname string
			var missingColumns string
			err = rows.Scan(&nspname, &relname, &indexname, &missingColumns)
			if err != nil {
				return xerrors.Errorf("database %v: %w", dbInfo.QuotedDatname, err)
			}

			databaseContents.WriteString(fmt.Sprintf("%s.%s does not contain the partitioning columns %s of table %s.%s\n",
				nspname, indexname, missingColumns, nspname, relname))
		}

		if err = rows.Err(); err != nil {
			return xerrors.Errorf("database %v: %w", dbInfo.QuotedDatname, err)
		}

		if databaseContents.Len() == 0 {
			continue
		}

		indexesArePresent = true

		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")
		contents.WriteString(fmt.Sprintf("Database: %s\n", dbInfo.QuotedDatname))
		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")
		contents.Write(databaseContents.Bytes())
	}

	if !indexesArePresent {
		// no such index in the cluster, can safely continue
		return nil
	}

	outputDir, err := utils.GetLogDir()
	if err != nil {
		return xerrors.Errorf("Internal error: unexpected condition")
	}

	filePath := filepath.Join(outputDir, OutputFileUnsupportedUniqueIndexes)
	err = utils.System.WriteFile(filePath, contents.Bytes(), 0644)
	if err != nil {
		return err
	}

	return xerrors.Errorf(ErrorMessageUnsupportedUniqueIndexes, filePath)
}
