#!/usr/bin/env bash
# Copyright (c) 2017-2023 VMware, Inc. or its affiliates
# SPDX-License-Identifier: Apache-2.0

GGUPGRADE_SOURCE_PATH=/vagrant

STANDBY_OPTS="-s standby-agent.local"
GP_INIT_SYSTEM="cd $GGUPGRADE_SOURCE_PATH/multihost && gpinitsystem -a -c ggupgrade_cluster_config ${STANDBY_OPTS}"

vagrant ssh hub --command="$GP_INIT_SYSTEM"
