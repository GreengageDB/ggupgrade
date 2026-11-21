// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/utils"
)

func MustAddTablespace(t *testing.T, cluster greengage.Cluster, tablespaceDir string) {
	t.Helper()

	if cluster.Version.Major == 5 {
		MustCreateFilespaceAndTablespace(t, cluster, tablespaceDir)
	}

	if cluster.Version.Major >= 6 {
		MustCreateTablespace(t, cluster, tablespaceDir)
	}

	// create databases in the tablespace
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), `CREATE DATABASE foodb TABLESPACE test_tablespace;`)
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), `CREATE DATABASE eatdb TABLESPACE test_tablespace;`)
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), `CREATE DATABASE boodb TABLESPACE test_tablespace2;`)

	// create tables in the tablespace
	sql := `
CREATE TABLE public.tablespace_table_0 (a int) TABLESPACE test_tablespace;
INSERT INTO public.tablespace_table_0 SELECT i from generate_series(1,100)i;

CREATE TABLE public.tablespace_table_1 (a int) WITH(appendonly=true, orientation=row) TABLESPACE test_tablespace;
INSERT INTO public.tablespace_table_1 SELECT i from generate_series(1,100)i;

CREATE TABLE public.tablespace_table_2 (a int) WITH(appendonly=true, orientation=column) TABLESPACE test_tablespace;
INSERT INTO public.tablespace_table_2 SELECT i from generate_series(1,100)i;

CREATE TABLE  public.tablespace_table_3 (a int, b int) WITH(appendonly=true, orientation=column) TABLESPACE test_tablespace
PARTITION BY RANGE(b) (START(1) END(4) EVERY(1));
INSERT INTO public.tablespace_table_3 SELECT i, (i%3)+1 FROM generate_series(1,100)i;

CREATE TABLE public.tablespace_table_4 (a int) WITH(appendonly=true, orientation=column) TABLESPACE test_tablespace2;
INSERT INTO public.tablespace_table_4 SELECT i from generate_series(1,100)i;`
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), sql)

	// add a table to the database within the tablespace
	sql = `
CREATE TABLE public.tablespace_table_0 (a int);
INSERT INTO public.tablespace_table_0 SELECT i from generate_series(1,100)i;`
	conn := cluster.Connection(greengage.Database("foodb"))
	MustExecuteSQL(t, conn, sql)

	// add a table to the database within the tablespace
	sql = `
CREATE TABLE public.tablespace_table_0 (a int);
INSERT INTO public.tablespace_table_0 SELECT i from generate_series(1,100)i;`
	conn = cluster.Connection(greengage.Database("eatdb"))
	MustExecuteSQL(t, conn, sql)

	// add a table to the database within the tablespace
	sql = `
CREATE TABLE public.tablespace_table_0 (a int);
INSERT INTO public.tablespace_table_0 SELECT i from generate_series(1,100)i;`
	conn = cluster.Connection(greengage.Database("boodb"))
	MustExecuteSQL(t, conn, sql)
}

func MustCreateTablespace(t *testing.T, cluster greengage.Cluster, tablespaceDir string) {
	t.Helper()

	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), fmt.Sprintf(`CREATE TABLESPACE test_tablespace LOCATION '%s';`, tablespaceDir))
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")),
		fmt.Sprintf(`CREATE TABLESPACE test_tablespace2 LOCATION '%s2' WITH (content1='%s3');`, tablespaceDir, tablespaceDir))
}

func MustCreateFilespaceAndTablespace(t *testing.T, cluster greengage.Cluster, tablespaceDir string) {
	t.Helper()

	var sb strings.Builder
	sb.WriteString("filespace:test_FS\n")
	for _, seg := range cluster.Primaries {
		// Keep symlinks short otherwise they get trimmed and result in an invalid symlink when pg_basebackup copies
		// the coordinator to standby.
		path := filepath.Join(tablespaceDir, "testfs", strconv.Itoa(seg.DbID), filepath.Base(seg.DataDir))
		MustCreateDirRemotely(t, seg.Hostname, filepath.Dir(path))
		sb.WriteString(fmt.Sprintf("%s:%d:%s\n", seg.Hostname, seg.DbID, path))
	}

	for _, seg := range cluster.Mirrors {
		// Keep symlinks short otherwise they get trimmed and result in an invalid symlink when pg_basebackup copies
		// the coordinator to standby.
		path := filepath.Join(tablespaceDir, "testfs", strconv.Itoa(seg.DbID), filepath.Base(seg.DataDir))
		MustCreateDirRemotely(t, seg.Hostname, filepath.Dir(path))
		sb.WriteString(fmt.Sprintf("%s:%d:%s\n", seg.Hostname, seg.DbID, path))
	}

	config := filepath.Join(tablespaceDir, "testfs", "fs.config")
	MustWriteToFile(t, config, sb.String())

	// gpfilespace requires the HOME environment variable
	err := cluster.RunGreengageCmdWithEnvironment(step.NewLogStdStreams(false), "gpfilespace", []string{"--config", config}, utils.FilterEnv([]string{"HOME"}))
	if err != nil {
		t.Fatal(err)
	}

	// create a tablespace in the filespace
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), `CREATE TABLESPACE test_tablespace FILESPACE test_FS;`)
}

// MustDeleteTablespaces deletes tablespaces from the target cluster. However,
// since we do not yet support upgrading tablespaces from version GPDB 6+ we do
// not need to delete them as they do not exist.
func MustDeleteTablespaces(t *testing.T, source greengage.Cluster, target greengage.Cluster) {
	t.Helper()

	conn := target.Connection(greengage.Database("foodb"))
	MustExecuteSQL(t, conn, `DROP TABLE IF EXISTS public.tablespace_table_0;`)

	conn = target.Connection(greengage.Database("eatdb"))
	MustExecuteSQL(t, conn, `DROP TABLE IF EXISTS public.tablespace_table_0;`)

	conn = target.Connection(greengage.Database("boodb"))
	MustExecuteSQL(t, conn, `DROP TABLE IF EXISTS public.tablespace_table_0;`)

	MustExecuteSQL(t, target.Connection(), `DROP DATABASE foodb;`)
	MustExecuteSQL(t, target.Connection(), `DROP DATABASE eatdb;`)
	MustExecuteSQL(t, target.Connection(), `DROP DATABASE boodb;`)

	sql := `
DROP TABLE IF EXISTS public.tablespace_table_0;
DROP TABLE IF EXISTS public.tablespace_table_1;
DROP TABLE IF EXISTS public.tablespace_table_2;
DROP TABLE IF EXISTS public.tablespace_table_3;
DROP TABLE IF EXISTS public.tablespace_table_4;
`
	MustExecuteSQL(t, target.Connection(greengage.Database("postgres")), sql)

	sql = `DROP TABLESPACE IF EXISTS test_tablespace;`
	if target.Version.Major == 5 {
		sql += `DROP FILESPACE IF EXISTS test_FS;`
	}

	MustExecuteSQL(t, target.Connection(greengage.Database("postgres")), sql)
	MustExecuteSQL(t, target.Connection(greengage.Database("postgres")), `DROP TABLESPACE IF EXISTS test_tablespace2;`)
}

func MustTruncateTablespaces(t *testing.T, cluster greengage.Cluster) {
	t.Helper()

	sql := `
TRUNCATE public.tablespace_table_0;
TRUNCATE public.tablespace_table_1;
TRUNCATE public.tablespace_table_2;
TRUNCATE public.tablespace_table_3;
TRUNCATE public.tablespace_table_4;
`
	MustExecuteSQL(t, cluster.Connection(greengage.Database("postgres")), sql)

	conn := cluster.Connection(greengage.Database("foodb"))
	MustExecuteSQL(t, conn, `TRUNCATE public.tablespace_table_0;`)

	conn = cluster.Connection(greengage.Database("eatdb"))
	MustExecuteSQL(t, conn, `TRUNCATE public.tablespace_table_0;`)

	conn = cluster.Connection(greengage.Database("boodb"))
	MustExecuteSQL(t, conn, `TRUNCATE public.tablespace_table_0;`)
}

func VerifyTablespaceData(t *testing.T, cluster greengage.Cluster) {
	t.Helper()

	tables := []string{"public.tablespace_table_0", "public.tablespace_table_1", "public.tablespace_table_2", "public.tablespace_table_3", "public.tablespace_table_4"}
	for _, table := range tables {
		rows := MustQueryRow(t, cluster.Connection(greengage.Database("postgres")), `SELECT COUNT(*) FROM `+table)
		expected := 100
		if rows != expected {
			t.Fatalf("got %v want %v rows", rows, expected)
		}
	}

	conn := cluster.Connection(greengage.Database("foodb"))
	row := MustQueryRow(t, conn, `SELECT COUNT(*) FROM public.tablespace_table_0;`)
	expectedCount := 100
	if row != expectedCount {
		t.Fatalf("got %d want %d rows", row, expectedCount)
	}

	conn = cluster.Connection(greengage.Database("eatdb"))
	row = MustQueryRow(t, conn, `SELECT COUNT(*) FROM public.tablespace_table_0;`)
	expectedCount = 100
	if row != expectedCount {
		t.Fatalf("got %d want %d rows", row, expectedCount)
	}

	conn = cluster.Connection(greengage.Database("boodb"))
	row = MustQueryRow(t, conn, `SELECT COUNT(*) FROM public.tablespace_table_0;`)
	expectedCount = 100
	if row != expectedCount {
		t.Fatalf("got %d want %d rows", row, expectedCount)
	}
}
