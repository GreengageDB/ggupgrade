// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package commanders_test

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/GreengageDB/ggupgrade/cli/commanders"
	"github.com/GreengageDB/ggupgrade/greengage"
	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/step"
	"github.com/GreengageDB/ggupgrade/substeps"
	"github.com/GreengageDB/ggupgrade/testutils"
	"github.com/GreengageDB/ggupgrade/testutils/exectest"
	"github.com/GreengageDB/ggupgrade/utils"
)

// UnsupportedIndex is a row of the query looking for unique indexes on partitioned tables that
// Greengage 7 does not allow.
type UnsupportedIndex struct {
	schema         string
	table          string
	index          string
	missingColumns string
}

type MockDatabaseWithIndexes struct {
	name       string
	quotedName string
	indexes    []UnsupportedIndex
}

func TestCheckForUnsupportedUniqueIndexes(t *testing.T) {
	//	Make sure that we always reset global state
	defer commanders.ResetBootstrapConnectionFunction()

	greengage.SetVersionCommand(exectest.NewCommand(PostgresGPVersion_6_7_1))
	defer greengage.ResetVersionCommand()

	t.Run("gracefully exits when the database query fails to return rows", func(t *testing.T) {
		dbTemplate1, mockTemplate1, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mockTemplate1, t)

		commanders.SetBootstrapConnectionFunction(func(destination idl.ClusterDestination, gphome string, port int, database string) (*sql.DB, error) {
			return dbTemplate1, nil
		})
		defer commanders.ResetBootstrapConnectionFunction()

		expectedErr := sql.ErrConnDone
		expectPgDatabaseToReturn(mockTemplate1).WillReturnError(expectedErr)
		mockTemplate1.ExpectClose()

		err = commanders.CheckForUnsupportedUniqueIndexes(step.DevNullStream, "", 0)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, err)
		}
	})

	t.Run("gracefully exits when the index query fails to return rows", func(t *testing.T) {
		dbTemplate1, mockTemplate1, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mockTemplate1, t)

		dbPostgres, mockPostgres, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}
		defer testutils.FinishMock(mockPostgres, t)

		commanders.SetBootstrapConnectionFunction(func(destination idl.ClusterDestination, gphome string, port int, database string) (*sql.DB, error) {
			if database == "template1" {
				return dbTemplate1, nil
			}

			return dbPostgres, nil
		})
		defer commanders.ResetBootstrapConnectionFunction()

		expectDatabaseQueryToReturnIndexes(mockTemplate1, []MockDatabaseWithIndexes{{name: "postgres", quotedName: "postgres"}})
		mockTemplate1.ExpectClose()

		expectedErr := sql.ErrConnDone
		expectUnsupportedIndexQuery(mockPostgres).WillReturnError(expectedErr)
		mockPostgres.ExpectClose()

		err = commanders.CheckForUnsupportedUniqueIndexes(step.DevNullStream, "", 0)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, err)
		}
	})

	t.Run("continues when no such index is present", func(t *testing.T) {
		MockCheckForUnsupportedUniqueIndexes(t, []MockDatabaseWithIndexes{
			{
				name:       "postgres",
				quotedName: "postgres",
				indexes:    []UnsupportedIndex{},
			},
		})
	})

	t.Run("reports the indexes and the partitioning columns they are missing", func(t *testing.T) {
		MockCheckForUnsupportedUniqueIndexes(t, []MockDatabaseWithIndexes{
			{
				name:       "postgres",
				quotedName: "postgres",
				indexes: []UnsupportedIndex{
					{schema: "s", table: "sales", index: "sales_unique_idx", missingColumns: "office_id"},
					{schema: "s", table: "ml", index: "ml_unique_idx", missingColumns: "office_id, dummy"},
				},
			},
			{
				name:       "another database",
				quotedName: "\"another database\"",
				indexes: []UnsupportedIndex{
					{schema: "public", table: "sales", index: "sales_unique_idx", missingColumns: "office_id"},
				},
			},
		})
	})

	t.Run("reports only the databases that have such an index", func(t *testing.T) {
		MockCheckForUnsupportedUniqueIndexes(t, []MockDatabaseWithIndexes{
			{
				name:       "postgres",
				quotedName: "postgres",
				indexes:    []UnsupportedIndex{},
			},
			{
				name:       "testdb",
				quotedName: "testdb",
				indexes: []UnsupportedIndex{
					{schema: "s", table: "sales", index: "sales_unique_idx", missingColumns: "office_id"},
				},
			},
		})
	})
}

func MockCheckForUnsupportedUniqueIndexes(t *testing.T, databaseInfos []MockDatabaseWithIndexes) {
	t.Helper()

	dbTemplate1, mockTemplate1, err := sqlmock.New()
	if err != nil {
		t.Fatalf("couldn't create sqlmock: %v", err)
	}
	defer testutils.FinishMock(mockTemplate1, t)

	databaseMapping := make(map[string]*sql.DB)
	databaseMapping["template1"] = dbTemplate1

	var databaseMocks []sqlmock.Sqlmock
	errorIsExpected := false
	for _, info := range databaseInfos {
		db, mock, mErr := sqlmock.New()
		if mErr != nil {
			t.Fatalf("couldn't create sqlmock: %v", mErr)
		}

		expectUnsupportedIndexQueryToReturn(mock, info.indexes)
		if len(info.indexes) > 0 {
			errorIsExpected = true
		}
		mock.ExpectClose()

		databaseMocks = append(databaseMocks, mock)
		databaseMapping[info.name] = db
	}
	defer func() {
		for _, mock := range databaseMocks {
			testutils.FinishMock(mock, t)
		}
	}()

	commanders.SetBootstrapConnectionFunction(func(destination idl.ClusterDestination, gphome string, port int, database string) (*sql.DB, error) {
		db, ok := databaseMapping[database]
		if !ok {
			return nil, fmt.Errorf("Internal test failure: no database with the name %v was created", database)
		}
		return db, nil
	})
	defer commanders.ResetBootstrapConnectionFunction()

	expectDatabaseQueryToReturnIndexes(mockTemplate1, databaseInfos)
	mockTemplate1.ExpectClose()

	indexErr := commanders.CheckForUnsupportedUniqueIndexes(step.DevNullStream, "", 0)
	if !errorIsExpected {
		if indexErr != nil {
			t.Fatalf("Unexpected error.\n%v", indexErr)
		}

		// No such index in the cluster. Everything is ok.
		return
	}

	outputDir, err := utils.GetLogDir()
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(outputDir, commanders.OutputFileUnsupportedUniqueIndexes)

	// CheckForUnsupportedUniqueIndexes doesn't specify error types, so let's compare their messages
	expectedErr := fmt.Sprintf(commanders.ErrorMessageUnsupportedUniqueIndexes, filePath)
	if indexErr == nil || expectedErr != indexErr.Error() {
		t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, indexErr)
	}

	actualOutputBytes, err := utils.System.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file")
	}

	expectedOutput := formatIndexOutputFile(databaseInfos)
	actualOutput := string(actualOutputBytes)
	if expectedOutput != actualOutput {
		t.Fatalf("Unexpected output.\nExpected:\n%v\nGot:\n%v", expectedOutput, actualOutput)
	}
}

// We have to do a little bit of code duplication, because mock.ExpectQuery
// does regular expression matching rather than string comparison

func expectUnsupportedIndexQuery(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(`WITH partition_keys AS`)
}

func expectUnsupportedIndexQueryToReturn(mock sqlmock.Sqlmock, indexes []UnsupportedIndex) *sqlmock.ExpectedQuery {
	rows := sqlmock.NewRows([]string{"schemaname", "tablename", "indexname", "missing_columns"})
	for _, index := range indexes {
		rows.AddRow(index.schema, index.table, index.index, index.missingColumns)
	}

	return expectUnsupportedIndexQuery(mock).WillReturnRows(rows)
}

func expectDatabaseQueryToReturnIndexes(mock sqlmock.Sqlmock, databases []MockDatabaseWithIndexes) {
	// We assume that datname and quoted datname are the same string,
	// which is not true. In any case, database names in this test are arbitrary
	rows := sqlmock.NewRows([]string{"datname", "quoted_datname"})
	for _, database := range databases {
		rows.AddRow(database.name, database.quotedName)
	}
	expectPgDatabaseToReturn(mock).WillReturnRows(rows)
}

// Formatting function that mirrors the one under test, so that the contents of the report file are
// checked rather than assumed.
func formatIndexOutputFile(databaseInfos []MockDatabaseWithIndexes) string {
	var contents bytes.Buffer
	for _, info := range databaseInfos {
		if len(info.indexes) == 0 {
			continue
		}

		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")
		contents.WriteString(fmt.Sprintf("Database: %s\n", info.quotedName))
		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")

		for _, index := range info.indexes {
			contents.WriteString(fmt.Sprintf("%s.%s does not contain the partitioning columns %s of table %s.%s\n",
				index.schema, index.index, index.missingColumns, index.schema, index.table))
		}
	}

	return contents.String()
}
