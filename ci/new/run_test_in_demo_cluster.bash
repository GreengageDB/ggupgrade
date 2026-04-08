#!/bin/bash

docker run -i --rm gpdb7_ggupgrade:latest bash -ex <<EOF
    set -exuo pipefail
    gpdb_src/concourse/scripts/setup_gpadmin_user.bash
    su gpadmin -c '
        source /usr/local/greengage-db-6X/greengage_path.sh
        cd /home/gpadmin/ggdb6_src/
        make create-demo-cluster WITH_MIRRORS=${WITH_MIRRORS:-true} WITH_STANDBY=${WITH_STANDBY:-true}
    '

    su gpadmin <<EOF1
        source /usr/local/greengage-db-6X/greengage_path.sh
        source /home/gpadmin/ggdb6_src/gpAux/gpdemo/gpdemo-env.sh
        export GPHOME_SOURCE=/usr/local/greengage-db-6X/
        export GPHOME_TARGET=/usr/local/greengage-db-7X/
        export PGPORT=6000
        export PGHOST=/tmp/
        export PATH=\\\$PATH:/opt/go/bin:~/go/bin
        export GOPATH=~/go
        mkdir -p /home/gpadmin/go/bin
        cd /home/gpadmin/ggupgrade
        make
        make install
        $1
EOF1
EOF
