#!/bin/bash
set -eox pipefail

project="$1"

docker compose -p "$project" -f ci/new/docker-compose.yaml up -d

services=$(docker compose -p "$project" -f ci/new/docker-compose.yaml config --services | tr '\n' ' ')

# Prepare ALL containers first
for service in $services
do
  docker compose -p $project -f ci/new/docker-compose.yaml exec -T \
    $service bash -c " mkdir -p /data/primary && mkdir -p /data/mirror && mkdir -p /data/coordinator/ && mkdir -p /data/standby/seg-1 &&
      chmod -R 777 /data &&
      # each host should have its own copy of the (initially identical) files in .ssh
      cp -rf .ssh.src .ssh &&
      bash -c 'source gpdb_src/concourse/scripts/common.bash; install_and_configure_gpdb;' &&
      ./gpdb_src/concourse/scripts/setup_gpadmin_user.bash" &
done
wait

# Add host keys to known_hosts after containers setup
for service in $services
do
  docker compose -p $project -f ci/new/docker-compose.yaml exec -T \
    $service bash -c "ssh-keyscan ${services/$service/} >> /home/gpadmin/.ssh/known_hosts" &
done
wait

# Add ip and host names of all cluster nodes to /etc/hosts
for service in $services
do
  docker compose -p $project -f ci/new/docker-compose.yaml exec -T \
    $service bash -c "for HOST in $services; do echo \"\$(host \"\$HOST\" | grep 'has address' | head -n 1 | cut -d ' ' -f 4) \$HOST\" >>/etc/hosts; done" &
done
wait
