-- Copyright (c) 2017-2023 VMware, Inc. or its affiliates
-- SPDX-License-Identifier: Apache-2.0

-- generates a script to drop foreign key constraints.
-- Foreign key constraints have to be dropped before primary/unique constraints to make sure that
-- we can successfully drop the dependee constraints.
-- Note that we primary and unique constraints need to be created before
-- foreign key constraints such that they can be properly referenced referenced.
-- Thus, we place them in the same subdirectory.
SELECT
   -- IF EXISTS because "initialize" applies the scripts on every run and an
   -- earlier run may have dropped the constraint already.
   'ALTER TABLE ' || pg_catalog.quote_ident(nspname) || '.' || pg_catalog.quote_ident(relname) || ' DROP CONSTRAINT IF EXISTS ' || pg_catalog.quote_ident(conname) || ';'
-- Every unique and primary key constraint is dropped with CASCADE, which takes
-- the foreign keys referencing them along. So this has to cover every table,
-- not just the root partitions, otherwise a foreign key on an ordinary table is
-- dropped here without ever being recreated.
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
            -- dropping a constraint directly from a child partition is
            -- rejected, the root takes care of them
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
