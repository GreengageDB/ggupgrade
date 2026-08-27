-- Copyright (c) 2017-2023 VMware, Inc. or its affiliates
-- SPDX-License-Identifier: Apache-2.0

-- This script assumes that the plpython language is already enabled
-- The generator should enable it if necessary on each database prior to this point

-- TODO: This implementation can be greatly simplified using recursive CTEs
-- They will be supported for 6X -> 7X upgrades

SET client_min_messages TO WARNING;

DROP TABLE IF EXISTS  __ggupgrade_tmp_generator.__temp_views_list;

CREATE TABLE  __ggupgrade_tmp_generator.__temp_views_list (full_view_name TEXT, view_owner TEXT, view_order INTEGER);

INSERT INTO __ggupgrade_tmp_generator.__temp_views_list
WITH RECURSIVE
-- Views reading a column of one of the deprecated types directly from a table.
leaf_views AS
(
    SELECT DISTINCT
        v.oid
    FROM
        pg_catalog.pg_depend d
            JOIN pg_catalog.pg_rewrite r ON r.oid = d.objid
            JOIN pg_catalog.pg_class v ON v.oid = r.ev_class
            JOIN pg_catalog.pg_namespace nv ON nv.oid = v.relnamespace
            JOIN pg_catalog.pg_attribute a ON (a.attrelid = d.refobjid AND a.attnum = d.refobjsubid)
            JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
            JOIN pg_catalog.pg_namespace nc ON nc.oid = c.relnamespace
    WHERE
        v.relkind = 'v'
        AND d.classid = 'pg_rewrite'::regclass
        AND d.refclassid = 'pg_class'::regclass
        AND d.deptype = 'n'
        AND (a.atttypid = 'pg_catalog.tsquery'::pg_catalog.regtype OR a.atttypid = 'pg_catalog.name'::pg_catalog.regtype)
        AND c.relkind = 'r'
        AND NOT a.attisdropped
        AND nv.nspname NOT LIKE 'pg_temp_%'
        AND nv.nspname NOT LIKE 'pg_toast_temp_%'
        AND nv.nspname NOT IN ('pg_catalog', 'information_schema')
        AND nc.nspname NOT LIKE 'pg_temp_%'
        AND nc.nspname NOT LIKE 'pg_toast_temp_%'
        AND nc.nspname NOT IN ('pg_catalog', 'information_schema')
),
-- Views reading from another view.
view_dependencies AS
(
    SELECT DISTINCT
        v.oid AS depender,
        dv.oid AS dependee
    FROM
        pg_catalog.pg_rewrite r
            JOIN pg_catalog.pg_depend d ON d.objid = r.oid
            JOIN pg_catalog.pg_class v ON v.oid = r.ev_class
            JOIN pg_catalog.pg_namespace nv ON nv.oid = v.relnamespace
            JOIN pg_catalog.pg_class dv ON dv.oid = d.refobjid
            JOIN pg_catalog.pg_namespace ndv ON ndv.oid = dv.relnamespace
    WHERE
        d.classid = 'pg_rewrite'::regclass
        AND d.refclassid = 'pg_class'::regclass
        AND dv.relkind = 'v'
        -- A view always depends on itself through its own rewrite rule.
        AND v.oid <> dv.oid
        AND nv.nspname NOT LIKE 'pg_temp_%'
        AND nv.nspname NOT LIKE 'pg_toast_temp_%'
        AND nv.nspname NOT IN ('pg_catalog', 'information_schema', 'gp_toolkit')
        AND ndv.nspname NOT LIKE 'pg_temp_%'
        AND ndv.nspname NOT LIKE 'pg_toast_temp_%'
        AND ndv.nspname NOT IN ('pg_catalog', 'information_schema', 'gp_toolkit')
),
-- Walk the dependency graph upwards from the leaf views. The same view can be
-- reached through several chains, the longest one wins below.
view_chains (oid, view_order) AS
(
    SELECT
        oid,
        1
    FROM
        leaf_views
    UNION
    SELECT
        dep.depender,
        chain.view_order + 1
    FROM
        view_chains chain
            JOIN view_dependencies dep ON dep.dependee = chain.oid
)
SELECT
    nv.nspname || '.' || v.relname,
    pg_catalog.pg_get_userbyid(v.relowner),
    max(chain.view_order)
FROM
    view_chains chain
        JOIN pg_catalog.pg_class v ON v.oid = chain.oid
        JOIN pg_catalog.pg_namespace nv ON nv.oid = v.relnamespace
GROUP BY
    1, 2;
