#!/bin/bash

SQL_DUMP_URL=${SQL_DUMP_URL:-"https://github.com/GreengageDB/greengage/actions/runs/25043750463/artifacts/6682503262"}

function load_dump() {
    pushd /home/gpadmin
    wget $SQL_DUMP_URL

    unzip sqldump_ggdb6_ubuntu.zip
    artifact_archive_name=ubuntu_postgres_sqldump.tar
    tar -xf $artifact_archive_name
    if tar -xf $artifact_archive_name 2>/dev/null ; then
        rm -f $artifact_archive_name
        echo "SQL Dump $artifact_archive_name sucessfully unpacked and removed"
    else
        echo "::error::Artifact '$artifact_name' not found in any completed workflow run"
        exit 1
    fi

    echo "Loading SQL Dump"
    psql -d gptest -f sqldump/dump.sql 2>sql_load.log
    echo "SQL Dump load complete"

    popd
}

load_dump

gpcheckat -A

source gpupgrade_src/testutils/validate_mirrors_and_standby/validate_mirrors_and_standby.bash
validate_mirrors_and_standby /usr/local/greenplum-db-target cdw 5432