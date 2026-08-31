-- Copyright (c) 2017-2024 VMware, Inc. or its affiliates
-- SPDX-License-Identifier: Apache-2.0

--------------------------------------------------------------------------------
-- Create and setup non-upgradeable objects
--------------------------------------------------------------------------------

-- Greengage 7 requires a unique index on a partitioned table to contain every
-- partitioning column. Greengage 6 allows such an index, the initialize data
-- migration scripts drop it and the finalize ones replay its definition, which
-- the target cluster rejects. Nothing can recreate it there, so the upgrade has
-- to stop before anything is dropped.
CREATE SCHEMA unsupported_unique_index;
SET search_path TO unsupported_unique_index;

CREATE TABLE sales (trans_id int, office_id int, region text)
    DISTRIBUTED BY (trans_id)
    PARTITION BY RANGE (office_id) (START (1) END (4) EVERY (1));
CREATE UNIQUE INDEX sales_unique_idx ON sales(trans_id);

-- a multi level partitioned table is missing the partitioning columns of every level
CREATE TABLE ml_sales (trans_id int, office_id int, region int, dummy int)
    DISTRIBUTED BY (trans_id)
    PARTITION BY RANGE (office_id)
        SUBPARTITION BY RANGE (dummy)
            SUBPARTITION TEMPLATE (START (1) END (16) EVERY (4))
        (START (1) END (3) EVERY (1));
CREATE UNIQUE INDEX ml_sales_unique_idx ON ml_sales(trans_id);

RESET search_path;

--------------------------------------------------------------------------------
-- Assert that ggupgrade correctly detects the non-upgradeable objects
--------------------------------------------------------------------------------
!\retcode ggupgrade initialize --source-gphome="${GPHOME_SOURCE}" --target-gphome=${GPHOME_TARGET} --source-master-port=${PGPORT} --disk-free-ratio 0 --non-interactive;
! cat ~/gpAdminLogs/ggupgrade/partitioned_tables_with_unsupported_unique_indexes.txt;

--------------------------------------------------------------------------------
-- Workaround to unblock upgrade
--------------------------------------------------------------------------------
DROP SCHEMA unsupported_unique_index CASCADE;
