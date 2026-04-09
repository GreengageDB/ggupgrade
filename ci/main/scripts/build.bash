#!/bin/bash
# Copyright (c) 2017-2023 VMware, Inc. or its affiliates
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

cd ggupgrade_src
export GOFLAGS="-mod=readonly" # do not update dependencies during build
git fetch --tags

make oss-rpm
ci/main/scripts/verify-rpm.bash ggupgrade-*.rpm "Open Source"
mv ggupgrade-*.rpm ../built_oss
