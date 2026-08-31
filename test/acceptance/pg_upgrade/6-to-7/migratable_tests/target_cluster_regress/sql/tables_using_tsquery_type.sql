-- Copyright (c) 2017-2023 VMware, Inc. or its affiliates
-- SPDX-License-Identifier: Apache-2.0

--------------------------------------------------------------------------------
-- Check migratable objects
--------------------------------------------------------------------------------

SET search_path TO migratable_tsquery;

-- check data
SELECT * FROM table_with_tsquery_datatype_columns ORDER BY d;
SELECT * FROM table_with_tsquery ORDER BY name;

-- check that every column is a tsquery again
SELECT c.relname AS table_name,
       a.attname AS column_name,
       pg_catalog.format_type(a.atttypid, a.atttypmod) AS column_type
FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
WHERE n.nspname = 'migratable_tsquery'
    AND c.relkind IN ('r', 'p')
    AND a.attnum > 0
    AND NOT a.attisdropped
    AND a.atttypid = 'pg_catalog.tsquery'::pg_catalog.regtype
ORDER BY 1, 2;

-- check indexes on the tsquery columns
SELECT c.relname AS table_name, i.relname AS index_name
FROM pg_catalog.pg_index x
    JOIN pg_catalog.pg_class c ON c.oid = x.indrelid
    JOIN pg_catalog.pg_class i ON i.oid = x.indexrelid
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'migratable_tsquery'
ORDER BY 1, 2;

-- check the comment on the clustered index survived
SELECT pg_catalog.obj_description('migratable_tsquery.tsquery_cluster_comment_idx'::regclass, 'pg_class');

-- check views
SELECT viewname FROM pg_catalog.pg_views WHERE schemaname = 'migratable_tsquery' ORDER BY 1;

-- check the objects are usable, including the views stacked on other views
INSERT INTO table_with_tsquery VALUES ('kazbek', 'e & f'::tsquery);
SELECT * FROM view_on_tsquery ORDER BY name;
-- sort on the text of the tsquery, the sort order of the type itself follows
-- its internal representation.
SELECT * FROM view_on_view_and_table ORDER BY name, altitude::text;

RESET search_path;
