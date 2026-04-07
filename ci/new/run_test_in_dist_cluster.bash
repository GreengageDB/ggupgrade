#!/bin/bash
set -exuo pipefail

function exec_on_coordinator {
    cmd="$1"
    docker compose -p ggupgrade -f ci/new/docker-compose.yaml exec -u gpadmin -T coordinator bash -c "$cmd"
}

function cleanup {
    docker compose -p ggupgrade -f ci/new/docker-compose.yaml down
}

WITH_MIRRORS="${WITH_MIRRORS:-true}"
WITH_STANDBY="${WITH_STANDBY:-true}"
STANDBY_INIT_OPTS=""

mkdir ssh_keys -p
if [ ! -e "ssh_keys/id_rsa" ]
then
  ssh-keygen -P "" -f ssh_keys/id_rsa
fi

export IMAGE=gpdb7_ggupgrade:latest

bash ci/new/init_containers.sh "ggupgrade"

trap cleanup EXIT

exec_on_coordinator "bash -c
    cat > hostfile_all_hosts << 'EOF'
coordinator
standby
sdw1
sdw2
sdw3
EOF
"
exec_on_coordinator "cat hostfile_all_hosts"

exec_on_coordinator "bash -c
    cat > hostfile_segment_hosts << 'EOF'
sdw1
sdw2
sdw3
EOF
"

exec_on_coordinator "bash -c
    cat > init_config << 'EOF'
ARRAY_NAME=\"Greengage DB cluster\"
SEG_PREFIX=gpseg
PORT_BASE=10000
declare -a DATA_DIRECTORY=(/data/primary)
MASTER_HOSTNAME=coordinator
MASTER_DIRECTORY=/data/coordinator
MASTER_PORT=5432
TRUSTED_SHELL=ssh
CHECK_POINT_SEGMENTS=8
ENCODING=UNICODE
EOF
"

if [ "${WITH_MIRRORS}" == "true" ]; then
    exec_on_coordinator "bash -c
        cat >> init_config << 'EOF'
MIRROR_PORT_BASE=10500
declare -a MIRROR_DATA_DIRECTORY=(/data/mirror)
EOF
"
fi

if [ "${WITH_STANDBY}" == "true" ]; then
    STANDBY_INIT_OPTS="-s standby"
fi

set +e
# gpinitsystem can exit with non zero exit code
exec_on_coordinator "source /usr/local/greengage-db-6X/greengage_path.sh; gpinitsystem -a -c init_config -h hostfile_segment_hosts $STANDBY_INIT_OPTS"
set -e

exec_on_coordinator "
    source /usr/local/greengage-db-6X/greengage_path.sh;
    export GPHOME_SOURCE=/usr/local/greengage-db-6X/
    export GPHOME_TARGET=/usr/local/greengage-db-devel/
    export PGPORT=5432
    export PGHOST=/tmp/
    export PATH=\$PATH:/opt/go/bin:~/go/bin
    export GOPATH=~/go
    mkdir -p /home/gpadmin/go/bin
    cd /home/gpadmin/ggupgrade
    make
    make install
    $1
"
