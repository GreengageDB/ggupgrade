package upgrade

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"

	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/utils"
	"golang.org/x/xerrors"
)

// RelinkTablespaces read tablespace links from pg_tblspc and link them to the tablespace directory
// with the appropriate dbid.
func RelinkTablespaces(oldDBID string, newDataDir string, tablespaces map[int32]*idl.TablespaceInfo) error {
	dbid, err := strconv.Atoi(oldDBID)
	if err != nil {
		return err
	}

	tablespaceDir := fmt.Sprintf("%s/pg_tblspc", newDataDir)
	tablespacesDirs, err := utils.System.ReadDirFS(utils.System.DirFS(tablespaceDir), ".")
	if err != nil {
		return xerrors.Errorf("read tablespace directory: %w", err)
	}
	for _, oidDir := range tablespacesDirs {
		if oidDir.Type() != fs.ModeSymlink {
			return xerrors.Errorf("the file in the pg_tblspc is not a symlink: %s", oidDir.Name())
		}

		oid, err := strconv.Atoi(oidDir.Name())
		if err != nil {
			return err
		}

		tablespace, ok := tablespaces[int32(oid)]
		if !ok {
			return xerrors.Errorf("the file in the pg_tblspc is not a table space: %d", oid)
		}

		symLinkName := filepath.Join(tablespaceDir, oidDir.Name())
		targetDir := filepath.Join(tablespace.Location, strconv.Itoa(dbid))

		if err := utils.RecreateSymlink(targetDir, symLinkName); err != nil {
			return xerrors.Errorf("recreate symbolic link: %w", err)
		}
	}
	return nil
}
