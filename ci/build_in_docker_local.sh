#!/bin/bash
# FILE:    ci/build_in_docker_local.sh
# CONTEXT: Build ggupgrade and package it as a .deb in container
# PURPOSE: Convenience wrapper for local development. Runs
#          ci/build_in_docker.sh in a selected Greengage developer image.
#
# USAGE:   ci/build_in_docker_local.sh [UBUNTU_VERSION]
# EXAMPLE: ci/build_in_docker_local.sh 22.04

set -euo pipefail

target_os_version=${1:-22.04}

if [[ "$target_os_version" == "22.04" ]]; then
    os_image=ubuntu
else
    os_image="ubuntu${target_os_version}"
fi

image="ghcr.io/greengagedb/greengage/ggdb6_${os_image}:latest"
SRC=/ggupgrade/src

echo "Building ggupgrade on Ubuntu ${target_os_version}"

docker run --rm -it \
    -v "./:$SRC" -w "$SRC" \
    "$image" ci/build_in_docker.sh
