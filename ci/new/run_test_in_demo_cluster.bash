#!/bin/bash

docker run -it --rm gpdb7_ggupgrade:latest bash -c "
    set -exuo pipefail
    gpdb_src/concourse/scripts/setup_gpadmin_user.bash
    su gpadmin -c '
        source /home/gpadmin/ggupgrade/ci/new/ggdb6_bin/greengage_path.sh;
        cd /home/gpadmin/ggupgrade/ci/new/ggdb6_src/
        make create-demo-cluster WITH_MIRRORS=${WITH_MIRRORS:-true}
    '

    bash -c 'source gpdb_src/concourse/scripts/common.bash; install_and_configure_gpdb;'

    su gpadmin -c '
        source /home/gpadmin/ggupgrade/ci/new/ggdb6_bin/greengage_path.sh
        source /home/gpadmin/ggupgrade/ci/new/ggdb6_src/gpAux/gpdemo/gpdemo-env.sh;
        export GPHOME_SOURCE=/home/gpadmin/ggupgrade/ci/new/ggdb6_bin/
        export GPHOME_TARGET=/usr/local/greengage-db-devel/
        export PGPORT=6000
        export PGHOST=/tmp/
        export PATH=\$PATH:/opt/go/bin:~/go/bin
        export GOPATH=~/go
        mkdir -p /home/gpadmin/go/bin
        cd /home/gpadmin/ggupgrade
        make
        make install
        $1
    '
"
