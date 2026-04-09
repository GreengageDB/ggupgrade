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

export IMAGE=gpdb7_ggupgrade:latest

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

export GPHOME_SOURCE=/usr/local/greengage-db-6X/
export GPHOME_TARGET=/usr/local/greengage-db-7X/
export PGPORT=5432
export PGHOST=/tmp/
export PATH=\$PATH:/opt/go/bin:~/go/bin
export GOPATH=~/go
export ISOLATION2_PATH=/home/gpadmin/gpdb_src/src/test/isolation2
set +e
cd /home/gpadmin/ggupgrade
# run test command passed to script
$@
exit_code=\$?
set -e

params=(
  "./ d gpAdminLogs"
  "/data d log"
  "/data d pg_log"
)
for param in "\${params[@]}"; do
  read -r path type name <<< "\$param"
  gpssh -f /home/gpadmin/hostfile_all_hosts "find \$path -name \$name -type \$type -exec tar -rf "/logs/\$name.tar" {} \;"
done

cp /home/gpadmin/ggupgrade/test/acceptance/pg_upgrade/6-to-7/non_upgradeable_tests/regression.diffs /logs/regression_non_upgradeable.diffs || true
cp /home/gpadmin/ggupgrade/test/acceptance/pg_upgrade/6-to-7/upgradeable_tests/source_cluster_regress/regression.diffs /logs/regression_upgradeable.diffs || true
path="/home/gpadmin/ggupgrade/test/acceptance/pg_upgrade/6-to-7/"
name="results"
find \$path -name \$name -type d -exec tar -rf "/logs/\$name.tar" {} \; || true

exit \$exit_code
EOF
