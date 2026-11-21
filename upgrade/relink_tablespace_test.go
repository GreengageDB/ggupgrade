package upgrade_test

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"testing"

	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/upgrade"
	"github.com/GreengageDB/ggupgrade/utils"
	"golang.org/x/xerrors"
)

type MockDirEntry struct {
	name string
	mode fs.FileMode
	info fs.FileInfo
}

func (e MockDirEntry) Name() string {
	return e.name
}

func (e MockDirEntry) IsDir() bool {
	return e.mode.IsDir()
}

func (e MockDirEntry) Type() fs.FileMode {
	return e.mode.Type()
}

func (e MockDirEntry) Info() (fs.FileInfo, error) {
	return e.info, nil
}

func TestRelinkTablespace(t *testing.T) {
	t.Run("errors when parse dbID fails", func(t *testing.T) {
		var expected *strconv.NumError
		err := upgrade.RelinkTablespaces("", "", nil)
		if !errors.As(err, &expected) {
			t.Errorf("got error type %T want %T", err, expected)
		}
	})

	t.Run("Error when read list of tablespaces", func(t *testing.T) {
		expected := errors.New("oops")
		utils.System.ReadDirFS = func(fsys fs.FS, name string) ([]fs.DirEntry, error) {
			if fsys != os.DirFS("/datadir/pg_tblspc") {
				t.Error()
			}
			return nil, expected
		}
		defer utils.ResetSystemFunctions()

		err := upgrade.RelinkTablespaces("1", "/datadir", nil)
		if !errors.Is(err, expected) {
			t.Errorf("got %#v want %#v", err, expected)
		}
	})

	t.Run("Not a symlink in the tablespace directory", func(t *testing.T) {
		expected := xerrors.Errorf("the file in the pg_tblspc is not a symlink: 16000")
		utils.System.ReadDirFS = func(fsys fs.FS, name string) ([]fs.DirEntry, error) {
			if fsys != os.DirFS("/datadir/pg_tblspc") {
				t.Error()
			}

			tablespaces := make([]fs.DirEntry, 1)
			tablespaces[0] = MockDirEntry{
				name: "16000",
				mode: 0,
			}
			return tablespaces, nil
		}
		defer utils.ResetSystemFunctions()
		err := upgrade.RelinkTablespaces("1", "/datadir", nil)
		if expected.Error() != err.Error() {
			t.Errorf("got %#v want %#v", err, expected)
		}
	})

	t.Run("Link is not a number", func(t *testing.T) {
		utils.System.ReadDirFS = func(fsys fs.FS, name string) ([]fs.DirEntry, error) {
			if fsys != os.DirFS("/datadir/pg_tblspc") {
				t.Error()
			}

			tablespaces := make([]fs.DirEntry, 1)
			tablespaces[0] = MockDirEntry{
				name: "test",
				mode: os.ModeSymlink,
			}
			return tablespaces, nil
		}
		defer utils.ResetSystemFunctions()
		err := upgrade.RelinkTablespaces("1", "/datadir", nil)
		var expected *strconv.NumError
		if errors.Is(err, expected) {
			t.Errorf("got %#v want %#v", err, expected)
		}
	})

	t.Run("The tablespace oid is not in tablespace map", func(t *testing.T) {
		utils.System.ReadDirFS = func(fsys fs.FS, name string) ([]fs.DirEntry, error) {
			if fsys != os.DirFS("/datadir/pg_tblspc") {
				t.Error()
			}

			tablespaces := make([]fs.DirEntry, 1)
			tablespaces[0] = MockDirEntry{
				name: "16000",
				mode: os.ModeSymlink,
			}
			return tablespaces, nil
		}

		tablespaces := make(map[int32]*idl.TablespaceInfo)

		defer utils.ResetSystemFunctions()
		err := upgrade.RelinkTablespaces("1", "/datadir", tablespaces)
		expected := xerrors.Errorf("the file in the pg_tblspc is not a table space: %d", 16000)
		if errors.Is(err, expected) {
			t.Errorf("got %#v want %#v", err, expected)
		}
	})

	t.Run("errors when recreating the symlink fails to create the link 7X", func(t *testing.T) {
		expected := errors.New("oops")
		utils.System.ReadDirFS = func(fsys fs.FS, name string) ([]fs.DirEntry, error) {
			if fsys != os.DirFS("/datadir/pg_tblspc") {
				t.Error()
			}

			tablespaces := make([]fs.DirEntry, 1)
			tablespaces[0] = MockDirEntry{
				name: "16000",
				mode: os.ModeSymlink,
			}
			return tablespaces, nil
		}

		utils.System.Lstat = func(name string) (os.FileInfo, error) {
			return nil, nil
		}
		utils.System.Remove = func(name string) error {
			return nil
		}
		utils.System.Symlink = func(sourceDir, symLinkName string) error {
			return expected
		}

		defer utils.ResetSystemFunctions()

		tablespaces := map[int32]*idl.TablespaceInfo{
			16000: {
				Location:    "/tmp/ts1",
				Name:        "ts1",
				UserDefined: true,
			},
		}

		err := upgrade.RelinkTablespaces("1", "/datadir", tablespaces)
		if !errors.Is(err, expected) {
			t.Errorf("got %#v want %#v", err, expected)
		}
	})
}
