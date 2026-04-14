#!/bin/bash
set -exuo pipefail

function cleanup {
    docker compose -p ggupgrade -f ci/new/docker-compose.yaml down
}

WITH_MIRRORS="${WITH_MIRRORS:-true}"
WITH_STANDBY="${WITH_STANDBY:-true}"

mkdir ssh_keys -p
if [ ! -e "ssh_keys/id_rsa" ]
then
  ssh-keygen -P "" -f ssh_keys/id_rsa
fi

bash ci/new/init_containers.sh "ggupgrade"

trap cleanup EXIT

docker compose -p ggupgrade -f ci/new/docker-compose.yaml exec -u gpadmin -T coordinator bash -x <<EOF
set -exuo pipefail
export USER=gpadmin

cat > hostfile_all_hosts <<EOF1
coordinator
standby
sdw1
sdw2
sdw3
EOF1

cat > hostfile_segment_hosts << EOF1
sdw1
sdw2
sdw3
EOF1

cat > init_config << EOF1
ARRAY_NAME="Greengage DB cluster"
SEG_PREFIX=gpseg
PORT_BASE=10000
declare -a DATA_DIRECTORY=(/data/primary)
MASTER_HOSTNAME=coordinator
MASTER_DIRECTORY=/data/coordinator
MASTER_PORT=5432
TRUSTED_SHELL=ssh
CHECK_POINT_SEGMENTS=8
ENCODING=UNICODE
EOF1

if [ "${WITH_MIRRORS}" == "true" ]; then
    cat >> init_config << EOF1
MIRROR_PORT_BASE=10500
declare -a MIRROR_DATA_DIRECTORY=(/data/mirror)
EOF1
fi

STANDBY_INIT_OPTS=""
if [ "${WITH_STANDBY}" == "true" ]; then
    STANDBY_INIT_OPTS="-s standby"
fi

source /usr/local/greengage-db-6X/greengage_path.sh
gpinitsystem -a -c init_config -h hostfile_segment_hosts \$STANDBY_INIT_OPTS </dev/null || true

source ggupgrade/ci/new/common.bash
export PGPORT=5432

set +e
pushd ggupgrade
# run test command passed to script
$@
exit_code=\$?
set -e
popd

collect_logs /data gpssh -f hostfile_all_hosts

exit \$exit_code
EOF
