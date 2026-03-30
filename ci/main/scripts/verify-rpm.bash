#!/bin/bash
# Copyright (c) 2017-2023 VMware, Inc. or its affiliates
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail -o errtrace

RPM=$1
RELEASE=$2
VERSION=$(git describe --tags --abbrev=0)

verify_gpugprade_version_output() {
  [[ $(/usr/local/bin/ggupgrade version) == *"Version: ${VERSION}"* ]]
  [[ $(/usr/local/bin/ggupgrade version) == *"Release: ${RELEASE}"* ]]
}

verify_rpm_info() {
  local info="$1"

  [[ $info == *"Name        : ggupgrade"* ]]
  [[ $info == *"Architecture: x86_64"* ]]
  [[ $info == *"Source RPM  : ggupgrade-${VERSION}-1"* ]]
  [[ $info == *"URL         : https://github.com/GreengageDB/ggupgrade"* ]]

  if [ "$RELEASE" = "Open Source" ]; then
      [[ $info == *"License     : Apache 2.0"* ]]
      [[ $info == *"Summary     : Greengage Database Upgrade"* ]]
      return
  fi

  [[ $info == *"License     : VMware Software EULA"* ]]
  [[ $info == *"Summary     : VMware Greenplum Upgrade"* ]]
}

verify_license_files() {
  local license_file="/usr/local/bin/greengage/ggupgrade/open_source_licenses.txt"
  [ -s "$license_file" ]

  [[ $(head -1 "$license_file") =~ open_source_licenses.txt ]]
  [[ $(head -3 "$license_file" | tail -1) == *"VMware Greenplum Upgrade ${VERSION}"* ]]
  [[ $(tail -1 "$license_file") =~ "GREENGAGEUPGRADE" ]]
}

main() {
  [ -f "$RPM" ]
  [ "$RELEASE" = "Enterprise" ] || [ "$RELEASE" = "Open Source" ]

  rpm -ivh "$RPM"
  verify_gpugprade_version_output
  verify_rpm_info "$(rpm -qi ggupgrade)"
  verify_license_files

  rpm -ev ggupgrade
}

log_error() {
  echo "Error: line $(caller): ${BASH_COMMAND}"

  echo "
Are the tags synced and up to date between origin and the remote? This script expects the latest tag.
If your dev pipeline is failing consider running:
  git fetch --tags origin
  git push --tags <yourRemoteName>
If you recently tagged and the prod pipeline failed consider running:
  git push --tags origin"
}

trap log_error ERR

main

