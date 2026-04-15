#!/bin/bash
set -exuo pipefail

mkdir ssh_keys -p
if [ ! -e "ssh_keys/id_rsa" ]
then
  ssh-keygen -P "" -f ssh_keys/id_rsa
fi

bash ci/new/init_containers.sh "ggupgrade"
