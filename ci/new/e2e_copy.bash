#!/bin/bash

SQL_DUMP_URL=${SQL_DUMP_URL:-"https://github.com/GreengageDB/greengage/actions/runs/25043750463/artifacts/6682503262"}

function load_dump() {
    echo "Loading SQL Dump"
    psql -d postgres -f ci/new/basic_sql_dump.sql 2>sql_load.log
    echo "SQL Dump load complete"
}

load_dump

gpcheckcat -A

source gpupgrade_src/testutils/validate_mirrors_and_standby/validate_mirrors_and_standby.bash
validate_mirrors_and_standby /usr/local/greenplum-db-target cdw 5432