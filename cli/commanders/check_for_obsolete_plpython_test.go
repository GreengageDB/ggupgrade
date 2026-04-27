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

func TestCheckForObsoletePlpython(t *testing.T) {
	//	Make sure that we always reset global state
	defer commanders.ResetBootstrapConnectionFunction()

	greengage.SetVersionCommand(exectest.NewCommand(PostgresGPVersion_6_7_1))
	defer greengage.ResetVersionCommand()

	t.Run("gracefully exits when database query fails to return rows", func(t *testing.T) {
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

		err = commanders.CheckForObsoletePlpython(step.DevNullStream, "", 0, "")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, err)
		}
	})

	t.Run("gracefully exits when language query fails to return rows", func(t *testing.T) {
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
			} else if database == "postgres" {
				return dbPostgres, nil
			} else {
				return nil, fmt.Errorf("Internal test failure: no database with the name %v was created", database)
			}
		})
		defer commanders.ResetBootstrapConnectionFunction()

		expectDatabaseQueryToReturn(mockTemplate1, []string{"postgres"})
		mockTemplate1.ExpectClose()

		expectedErr := sql.ErrConnDone
		expectLanguageQuery(mockPostgres).WillReturnError(expectedErr)
		mockPostgres.ExpectClose()

		err = commanders.CheckForObsoletePlpython(step.DevNullStream, "", 0, "")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, err)
		}
	})

	t.Run("gracefully exits when function query fails to return rows", func(t *testing.T) {
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
			} else if database == "postgres" {
				return dbPostgres, nil
			} else {
				return nil, fmt.Errorf("Internal test failure: no database with the name %v was created", database)
			}
		})
		defer commanders.ResetBootstrapConnectionFunction()

		expectDatabaseQueryToReturn(mockTemplate1, []string{"postgres"})
		mockTemplate1.ExpectClose()

		expectedErr := sql.ErrConnDone
		expectLanguageQueryToReturn(mockPostgres, true, true)
		expectFunctionQuery(mockPostgres).WillReturnError(expectedErr)
		mockPostgres.ExpectClose()

		err = commanders.CheckForObsoletePlpython(step.DevNullStream, "", 0, "")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, err)
		}
	})

	t.Run("continues when no plpython is present", func(t *testing.T) {
		databaseInfos := []MockDatabase{
			{
				name:                "postgres",
				plpythonuIsPresent:  false,
				plpython2uIsPresent: false,
				functions:           []FunctionSignature{},
			},
		}
		MockCheckForObsoletePlpython(t, databaseInfos)
	})

	t.Run("reports plpythonu functions", func(t *testing.T) {
		databaseInfos := []MockDatabase{
			{
				name:                "postgres",
				plpythonuIsPresent:  true,
				plpython2uIsPresent: false,
				functions: []FunctionSignature{
					{
						name:   "some_plpythonu_function_1",
						args:   "string",
						schema: "public",
					},
					{
						name:   "some_plpythonu_function_376",
						args:   "int",
						schema: "public",
					},
				},
			},
			{
				name:                "another database",
				plpythonuIsPresent:  true,
				plpython2uIsPresent: false,
				functions: []FunctionSignature{
					{
						name:   "some_plpythonu_function_1",
						args:   "string",
						schema: "public",
					},
					{
						name:   "some_plpythonu_function_376",
						args:   "int",
						schema: "public",
					},
				},
			},
		}
		MockCheckForObsoletePlpython(t, databaseInfos)
	})

	t.Run("reports plpython2u functions", func(t *testing.T) {
		databaseInfos := []MockDatabase{
			{
				name:                "postgres",
				plpythonuIsPresent:  false,
				plpython2uIsPresent: true,
				functions: []FunctionSignature{
					{
						name:   "some_plpython2u_function_1",
						args:   "string",
						schema: "public",
					},
					{
						name:   "some_plpython2u_function_376",
						args:   "int",
						schema: "public",
					},
				},
			},
			{
				name:                "another database",
				plpythonuIsPresent:  false,
				plpython2uIsPresent: true,
				functions: []FunctionSignature{
					{
						name:   "some_plpython2u_function_1",
						args:   "string",
						schema: "public",
					},
					{
						name:   "some_plpython2u_function_376",
						args:   "int",
						schema: "public",
					},
				},
			},
		}
		MockCheckForObsoletePlpython(t, databaseInfos)
	})

	t.Run("reports all functions", func(t *testing.T) {
		databaseInfos := []MockDatabase{
			{
				name:                "postgres",
				plpythonuIsPresent:  true,
				plpython2uIsPresent: true,
				functions: []FunctionSignature{
					{
						name:   "some_plpython2u_function_2",
						args:   "oid, oidvector",
						schema: "myschema",
					},
					{
						name:   "some_plpython2u_function_377",
						args:   "int2vector",
						schema: "myschema",
					},
					{
						name:   "some_plpythonu_function_1",
						args:   "string",
						schema: "public",
					},
					{
						name:   "some_plpythonu_function_376",
						args:   "int",
						schema: "public",
					},
				},
			},
			{
				name:                "another database",
				plpythonuIsPresent:  true,
				plpython2uIsPresent: true,
				functions: []FunctionSignature{
					{
						name:   "some_plpython2u_function_2",
						args:   "oid, oidvector",
						schema: "myschema",
					},
					{
						name:   "some_plpython2u_function_377",
						args:   "int2vector",
						schema: "myschema",
					},
					{
						name:   "some_plpythonu_function_1",
						args:   "string",
						schema: "public",
					},
					{
						name:   "some_plpythonu_function_376",
						args:   "int",
						schema: "public",
					},
				},
			},
		}
		MockCheckForObsoletePlpython(t, databaseInfos)
	})

	t.Run("reports functions from muliple databases", func(t *testing.T) {
		databaseInfos := []MockDatabase{
			{
				name:                "postgres",
				plpythonuIsPresent:  false,
				plpython2uIsPresent: true,
				functions: []FunctionSignature{
					{
						name:   "some_plpython2u_function_2",
						args:   "oid, oidvector",
						schema: "myschema",
					},
					{
						name:   "some_plpython2u_function_377",
						args:   "int2vector",
						schema: "myschema",
					},
				},
			},
			{
				name:                "another database",
				plpythonuIsPresent:  true,
				plpython2uIsPresent: false,
				functions: []FunctionSignature{
					{
						name:   "other_plpythonu_function_1",
						args:   "string",
						schema: "my schema",
					},
					{
						name:   "other_plpythonu_function_376",
						args:   "int",
						schema: "my schema",
					},
				},
			},
		}
		MockCheckForObsoletePlpython(t, databaseInfos)
	})
}

type MockDatabase struct {
	name                string
	plpythonuIsPresent  bool
	plpython2uIsPresent bool
	functions           []FunctionSignature
}

func MockCheckForObsoletePlpython(t *testing.T, databaseInfos []MockDatabase) {
	dbTemplate1, mockTemplate1, err := sqlmock.New()
	if err != nil {
		t.Fatalf("couldn't create sqlmock: %v", err)
	}
	defer testutils.FinishMock(mockTemplate1, t)

	errorIsExpected := false
	errorShouldContainPlpythonu := false
	errorShouldContainPlpython2u := false

	var databaseNames []string
	var databaseMocks []sqlmock.Sqlmock
	databaseMapping := make(map[string]*sql.DB)

	databaseMapping["template1"] = dbTemplate1
	for _, info := range databaseInfos {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("couldn't create sqlmock: %v", err)
		}

		expectLanguageQueryToReturn(mock, info.plpythonuIsPresent, info.plpython2uIsPresent)
		if info.plpythonuIsPresent || info.plpython2uIsPresent {
			errorIsExpected = true
			errorShouldContainPlpythonu = errorShouldContainPlpythonu || info.plpythonuIsPresent
			errorShouldContainPlpython2u = errorShouldContainPlpython2u || info.plpython2uIsPresent
			expectFunctionQueryToReturn(mock, info.functions)
		}
		mock.ExpectClose()

		databaseNames = append(databaseNames, info.name)
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

	expectDatabaseQueryToReturn(mockTemplate1, databaseNames)
	mockTemplate1.ExpectClose()

	plpythonErr := commanders.CheckForObsoletePlpython(step.DevNullStream, "", 0, "")
	if !errorIsExpected {
		if plpythonErr != nil {
			t.Fatalf("Unexpected error.\n%v", err)
		}

		// No errors when plpython and plpython2u are not present. Everything is ok.
		return
	}

	outputDir, err := utils.GetLogDir()
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(outputDir, commanders.OutputFilePlpython)

	// Check that the error is correct
	// CheckForObsoletePlpython doesn't specify error types, so let's compare their messages
	expectedErr := formatErrorMessagePlpython(
		t, errorShouldContainPlpythonu, errorShouldContainPlpython2u, filePath)

	if plpythonErr == nil || expectedErr != plpythonErr.Error() {
		t.Fatalf("Unexpected error.\nExpected:\n%v\nGot:\n%v", expectedErr, plpythonErr)
	}

	actualOutputBytes, err := utils.System.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file")
	}

	expectedOutput := formatOutputFile(databaseInfos)
	actualOutput := string(actualOutputBytes)
	if expectedOutput != actualOutput {
		t.Fatalf("Unexpected output.\nExpected:\n%v\nGot:\n%v", expectedOutput, actualOutput)
	}

	// Everything is ok.
}

// We have to do a little bit of code duplication, because mock.ExpectQuery
// does regular expression matching rather than string comparison

func expectLanguageQuery(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(`SELECT EXISTS\(SELECT \* FROM pg_catalog\.pg_language WHERE lanname = 'plpythonu'\) plpythonu, EXISTS\(SELECT \* FROM pg_catalog\.pg_language WHERE lanname = 'plpython2u'\) plpython2u;`)
}

func expectLanguageQueryToReturn(mock sqlmock.Sqlmock, plpythonu bool, plpython2u bool) *sqlmock.ExpectedQuery {
	rows := sqlmock.NewRows([]string{"plpythonu", "plpython2u"}).AddRow(plpythonu, plpython2u)
	return expectLanguageQuery(mock).WillReturnRows(rows)
}

type FunctionSignature struct {
	name   string
	args   string
	schema string
}

func expectFunctionQuery(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
	const functionQuery = `
SELECT quote_ident\(c\.proname\) proname, pg_catalog\.pg_get_function_arguments\(c\.oid\) args, quote_ident\(n\.nspname\) nspname
    FROM pg_catalog\.pg_proc c
    JOIN pg_catalog\.pg_language l ON c\.prolang = l\.oid
    JOIN pg_catalog\.pg_namespace n ON c\.pronamespace = n\.oid
    WHERE l\.lanname in \('plpythonu', 'plpython2u'\);
`

	return mock.ExpectQuery(functionQuery)
}

func expectFunctionQueryToReturn(mock sqlmock.Sqlmock, functions []FunctionSignature) *sqlmock.ExpectedQuery {
	rows := sqlmock.NewRows([]string{"proname", "args", "nspname"})
	for _, function := range functions {
		rows.AddRow(function.name, function.args, function.schema)
	}

	return expectFunctionQuery(mock).WillReturnRows(rows)
}

func expectDatabaseQueryToReturn(mock sqlmock.Sqlmock, databases []string) {
	// We assume that datname and quoted datname are the same string,
	// which is not true. In any case, database names in this test are arbitrary
	rows := sqlmock.NewRows([]string{"datname", "quoted_datname"})
	for _, database := range databases {
		rows.AddRow(database, database)
	}
	expectPgDatabaseToReturn(mock).WillReturnRows(rows)
}

// Formatting function that has been tested to work.
// It feels like unnecessary code duplication, but at the same time
// there is no other way to check that the error message itself is correct.
func formatErrorMessagePlpython(t *testing.T, plpythonuIsPresent bool, plpython2uIsPresent bool, filePath string) string {
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
		t.Fatalf("Internal error: unexpected condition")
	}

	return fmt.Sprintf(commanders.ErrorMessagePlpython, foundLanguages, filePath, dropCommand)
}

// The same thing as above
func formatOutputFile(databaseInfos []MockDatabase) string {
	var contents bytes.Buffer

	for _, info := range databaseInfos {
		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")
		contents.WriteString(fmt.Sprintf("Database: %s\n", info.name))
		contents.WriteString(substeps.Divider)
		contents.WriteString("\n")

		for _, function := range info.functions {
			signatureString := fmt.Sprintf("%s.%s(%s)\n", function.schema, function.name, function.args)
			contents.WriteString(signatureString)
		}
	}

	return contents.String()
}
