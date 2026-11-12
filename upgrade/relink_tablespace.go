package upgrade

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"

	"github.com/GreengageDB/ggupgrade/utils"
	"golang.org/x/xerrors"
)

// RelinkTablespaces read tablespace links from pg_tblspc and link them to the tablespace directory
// with the appropriate dbid.
func RelinkTablespaces(oldDBID string, newDataDir string) error {
	dbid, err := strconv.Atoi(oldDBID)
	if err != nil {
		return err
	}

	tablespaceDir := fmt.Sprintf("%s/pg_tblspc", newDataDir)
	tablespaces, err := utils.System.ReadDirFS(utils.System.DirFS(tablespaceDir), ".")
	if err != nil {
		return xerrors.Errorf("read tablespace directory: %w", err)
	}
	for _, spaceDir := range tablespaces {
		if spaceDir.Type() != fs.ModeSymlink {
			return xerrors.Errorf("the file in the pg_tblspc is not a symlink: %s", spaceDir.Name())
		}

		link, err := utils.System.ReadLink(filepath.Join(tablespaceDir, spaceDir.Name()))
		if err != nil {
			return xerrors.Errorf("read link: %w", err)
		}

		location := filepath.Dir(filepath.Clean(link))

		symLinkName := filepath.Join(tablespaceDir, spaceDir.Name())
		targetDir := filepath.Join(location, strconv.Itoa(dbid))

		if err := utils.RecreateSymlink(targetDir, symLinkName); err != nil {
			return xerrors.Errorf("recreate symbolic link: %w", err)
		}
	}
	return nil
}
