// Copyright (c) 2017-2023 VMware, Inc. or its affiliates
// SPDX-License-Identifier: Apache-2.0

package greengage

import (
	"database/sql"
	"encoding/csv"
	"io"
	"path/filepath"
	"strconv"

	"github.com/GreengageDB/ggupgrade/idl"
	"github.com/GreengageDB/ggupgrade/utils"
	"github.com/GreengageDB/ggupgrade/utils/errorlist"
	"golang.org/x/xerrors"
)

const tablespacesQuery5X = `
	SELECT
		fsedbid as dbid,
		upgrade_tablespace.oid as oid,
		spcname as name,
		case when is_user_defined_tablespace then location_with_oid else fselocation end as location,
		(is_user_defined_tablespace::int) as userdefined
	FROM (
			SELECT
				pg_tablespace.oid,
				*,
				(fselocation || '/' || pg_tablespace.oid) as location_with_oid,
				(spcname not in ('pg_default', 'pg_global'))  as is_user_defined_tablespace
			FROM pg_tablespace
			INNER JOIN pg_filespace_entry
			ON fsefsoid = spcfsoid
		) upgrade_tablespace`

// In 6X and newer we only need tablespace location on the coordinator
const tablespacesQuery6XAndNewer = `
select 
    dbid,
    pt.oid,
    spcname as name,
    pg_tablespace_location(pt.oid) as location,
    1 as userdefined
    from gp_dist_random('pg_tablespace') as pt
    join gp_segment_configuration as gsc on gsc.content = pt.gp_segment_id
    where spcname not in ('pg_default', 'pg_global') and role = 'p'
    union all
    select 
    dbid,
    pt.oid,
    spcname as name,
    pg_tablespace_location(pt.oid) as location,
    1 as userdefined from
    pg_tablespace pt, gp_segment_configuration
    where content = -1 and role = 'p' and spcname not in ('pg_default', 'pg_global');`

// map<tablespaceOid, tablespaceInfo>
type SegmentTablespaces map[int32]*idl.TablespaceInfo

// map<DbID, map<tablespaceOid, tablespaceInfo>>
type Tablespaces map[int32]SegmentTablespaces

// slice of tablespace rows from database
type TablespaceTuples []Tablespace

type Tablespace struct {
	DbId int32
	Oid  int32
	Info *idl.TablespaceInfo
}

func (t Tablespaces) GetCoordinatorTablespaces() SegmentTablespaces {
	return t[CoordinatorDbid]
}

func (s SegmentTablespaces) UserDefinedTablespacesLocations() []string {
	var dirs []string
	for _, tsInfo := range s {
		if !tsInfo.GetUserDefined() {
			continue
		}

		dirs = append(dirs, tsInfo.GetLocation())
	}

	return dirs
}

func GetTablespaceLocationForDbId(t *idl.TablespaceInfo, dbId int) string {
	return filepath.Join(t.GetLocation(), strconv.Itoa(dbId))
}

func GetCoordinatorTablespaceLocation(basePath string, oid int) string {
	return filepath.Join(basePath, strconv.Itoa(oid), strconv.Itoa(CoordinatorDbid))
}

func GetTablespaceTuples(db *sql.DB, version uint) (TablespaceTuples, error) {
	var query string
	if version == 5 {
		query = tablespacesQuery5X
	} else {
		query = tablespacesQuery6XAndNewer
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := rows.Close(); cErr != nil {
			err = errorlist.Append(err, cErr)
		}
	}()

	results := make([]Tablespace, 0)
	for rows.Next() {
		var ts Tablespace
		ts.Info = &idl.TablespaceInfo{}
		if err := rows.Scan(&ts.DbId, &ts.Oid, &ts.Info.Name, &ts.Info.Location, &ts.Info.UserDefined); err != nil {
			return nil, xerrors.Errorf("scanning pg_tablespace: %w", err)

		}

		results = append(results, ts)
	}

	if err := rows.Err(); err != nil {
		return nil, xerrors.Errorf("iterating pg_tablespace rows: %w", err)

	}

	return results, nil
}

// convert the database tablespace query result to internal structure
func NewTablespaces(tuples TablespaceTuples) Tablespaces {
	clusterTablespaceMap := make(Tablespaces)
	for _, t := range tuples {
		tablespaceInfo := idl.TablespaceInfo{
			Location:    t.Info.GetLocation(),
			Name:        t.Info.Name,
			UserDefined: t.Info.GetUserDefined()}
		if segTablespaceMap, ok := clusterTablespaceMap[t.DbId]; ok {
			segTablespaceMap[t.Oid] = &tablespaceInfo
			clusterTablespaceMap[t.DbId] = segTablespaceMap
		} else {
			segTablespaceMap := make(SegmentTablespaces)
			segTablespaceMap[t.Oid] = &tablespaceInfo
			clusterTablespaceMap[t.DbId] = segTablespaceMap
		}
	}

	return clusterTablespaceMap
}

// write the tuples returned from the database to a csv file
func (t TablespaceTuples) Write(w io.Writer) error {
	writer := csv.NewWriter(w)
	for _, tablespace := range t {
		line := []string{
			strconv.Itoa(int(tablespace.DbId)),
			strconv.Itoa(int(tablespace.Oid)),
			tablespace.Info.Name,
			tablespace.Info.GetLocation(),
			boolToStrInt(tablespace.Info.GetUserDefined())}
		if err := writer.Write(line); err != nil {
			return xerrors.Errorf("write record %q: %w", line, err)
		}
	}
	defer writer.Flush()
	return nil
}

func boolToStrInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// main function which does the following:
// 1. query the database to get tablespace information
// 2. write the tablespace information to a file
// 3. converts the tablespace information to an internal structure
func TablespacesFromDB5X(db *sql.DB, tablespacesFile string) (Tablespaces, error) {
	tablespaceTuples, err := GetTablespaceTuples(db, 5)
	if err != nil {
		return nil, xerrors.Errorf("retrieve tablespace information: %w", err)
	}

	file, err := utils.System.Create(tablespacesFile)
	if err != nil {
		return nil, xerrors.Errorf("create tablespace file %q: %w", tablespacesFile, err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil {
			err = errorlist.Append(err, cErr)
		}
	}()
	if err := tablespaceTuples.Write(file); err != nil {
		return nil, xerrors.Errorf("populate tablespace mapping file: %w", err)
	}

	return NewTablespaces(tablespaceTuples), nil
}

// main function which does the following:
// 1. query the database to get tablespace information
// 2. converts the tablespace information to an internal structure
func TablespacesFromDB6XAndNewer(db *sql.DB) (Tablespaces, error) {
	tablespaceTuples, err := GetTablespaceTuples(db, 6)
	if err != nil {
		return nil, xerrors.Errorf("retrieve tablespace information: %w", err)
	}

	return NewTablespaces(tablespaceTuples), nil
}

func MapSpcname2Oid(db *sql.DB) (map[string]int32, error) {
	rows, err := db.Query("select spcname, oid from pg_tablespace where spcname not in ('pg_default', 'pg_global');")
	if err != nil {
		return nil, err
	}

	result := make(map[string]int32)
	for rows.Next() {
		var spcname string
		var oid int32
		if err := rows.Scan(&spcname, &oid); err != nil {
			return nil, xerrors.Errorf("scanning pg_tablespace: %w", err)
		}
		result[spcname] = oid
	}
	return result, nil
}
