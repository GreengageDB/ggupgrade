-- Copyright (c) 2017-2023 VMware, Inc. or its affiliates
-- SPDX-License-Identifier: Apache-2.0

-- We have to create foreign key constraints after primary/unique constraints to make sure that
-- we can successfully reference them.
SELECT
    $$ALTER TABLE $$ || pg_catalog.quote_ident(nspname) || $$.$$ || pg_catalog.quote_ident(relname) ||
    $$ ADD CONSTRAINT $$ || pg_catalog.quote_ident(conname) || $$ $$ ||
    pg_catalog.pg_get_constraintdef(cc.oid, false)  || $$;$$
-- This has to cover the same tables as the matching drop script, otherwise a
-- foreign key it dropped is never recreated.
FROM
    pg_constraint cc
        JOIN
    (
        SELECT DISTINCT
            c.oid,
            n.nspname,
            c.relname
        FROM
            pg_catalog.pg_class c
                JOIN
            pg_catalog.pg_namespace n
            ON (n.oid = c.relnamespace)
        WHERE
            c.relkind = 'r'
            AND n.nspname NOT LIKE 'pg_%'
            AND n.nspname <> 'information_schema'
            AND NOT EXISTS
            (
               SELECT 1
               FROM
                  pg_catalog.pg_partition_rule p
               WHERE
                  p.parchildrelid = c.oid
            )
    )
        as sub
    ON sub.oid = cc.conrelid
WHERE
    cc.contype = 'f';
