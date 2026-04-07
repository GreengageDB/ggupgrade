#!/usr/bin/env bash
set -euo pipefail

docker run --rm \
 -v "$(dirname "$(realpath "$0")")/ggdb6_bin:/usr/local/greengage-db-devel" \
 -v "$(dirname "$(realpath "$0")")/ggdb6_src:/home/gpadmin/ggdb6_src" \
 ghcr.io/greengagedb/greengage/ggdb6_ubuntu:latest bash -c \
"
    source gpdb_src/concourse/scripts/common.bash
    install_and_configure_gpdb
    cp -r /home/gpadmin/gpdb_src/* /home/gpadmin/ggdb6_src/
"

docker build -t gpdb7_ggupgrade:latest -f ci/new/Dockerfile .
