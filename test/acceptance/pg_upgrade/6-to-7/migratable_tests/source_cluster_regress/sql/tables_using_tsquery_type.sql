-- Copyright (c) 2017-2023 VMware, Inc. or its affiliates
-- SPDX-License-Identifier: Apache-2.0

--------------------------------------------------------------------------------
-- Create and setup migratable objects
--------------------------------------------------------------------------------

-- Create the objects in a custom schema to ensure the data migration scripts
-- fully qualify them.
CREATE SCHEMA migratable_tsquery;
SET search_path TO migratable_tsquery;

-- partitioned table with tsquery columns
CREATE TABLE table_with_tsquery_datatype_columns(a tsquery, b tsquery, c tsquery, d int)
    PARTITION BY RANGE(d) (START(1) END(4) EVERY(1));
INSERT INTO table_with_tsquery_datatype_columns
    VALUES  ('b & c'::tsquery, 'b & c'::tsquery, 'b & c'::tsquery, 1),
            ('e & f'::tsquery, 'e & f'::tsquery, 'e & f'::tsquery, 2),
            ('x & y'::tsquery, 'x & y'::tsquery, 'x & y'::tsquery, 3);

-- composite index on tsquery columns
CREATE TABLE tsquery_composite(i int, j tsquery, k tsquery);
CREATE INDEX tsquery_composite_idx ON tsquery_composite(j, k);

-- gist index on a tsquery column
CREATE TABLE tsquery_gist(i int, j tsquery, k tsquery);
CREATE INDEX tsquery_gist_idx ON tsquery_gist USING gist(j);

-- clustered index with a comment on a tsquery column
CREATE TABLE tsquery_cluster_comment(i int, j tsquery);
CREATE INDEX tsquery_cluster_comment_idx ON tsquery_cluster_comment(j);
ALTER TABLE tsquery_cluster_comment CLUSTER ON tsquery_cluster_comment_idx;
COMMENT ON INDEX tsquery_cluster_comment_idx IS 'hello world';

-- inherited tsquery columns. Greengage 6 refuses to inherit from a partitioned
-- table, so the parent is a plain table.
CREATE TABLE tsquery_inherits_parent (a tsquery, b tsquery, c tsquery, d int);
CREATE TABLE tsquery_inherits (
    e      tsquery
) INHERITS (tsquery_inherits_parent);

-- views on a tsquery column
CREATE TABLE table_with_tsquery (
    name       text,
    altitude   tsquery
);
CREATE INDEX table_with_tsquery_tsquery_idx ON table_with_tsquery(altitude);
INSERT INTO table_with_tsquery VALUES ('everest', 'a & b'::tsquery), ('elbrus', 'c & d'::tsquery);
INSERT INTO table_with_tsquery VALUES ('long_tsquery', 'a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a & a'::tsquery);

CREATE VIEW view_on_tsquery AS SELECT * FROM table_with_tsquery;
CREATE VIEW view_on_tsquery_mult_tables AS SELECT t1.name, t2.altitude FROM table_with_tsquery t1, table_with_tsquery t2;

-- views stacked on top of the views above. They have to be dropped top down and
-- recreated bottom up. Note that view_on_view_and_table reads the tsquery column
-- both through a view and straight from the table, so the depth of the deepest
-- chain is what decides when it can be dropped.
CREATE VIEW view_on_view_on_tsquery AS SELECT * FROM view_on_tsquery;
CREATE VIEW view_on_view_and_table AS
    SELECT v.name, t.altitude FROM view_on_view_on_tsquery v, table_with_tsquery t;

-- redundant check to make sure the we handle
-- columns inside distribution policy correctly.
-- q has attnum 3, which equals to the segment count.
CREATE TABLE table_with_tsquery_column_attnum_equal_to_segment_count (a tsquery, b int, q tsquery) DISTRIBUTED BY (b);
CREATE INDEX table_with_tsquery_column_attnum_equal_to_segment_count_idx ON
    table_with_tsquery_column_attnum_equal_to_segment_count (q);
CREATE VIEW table_with_tsquery_column_attnum_equal_to_segment_count_view AS
    SELECT * FROM table_with_tsquery_column_attnum_equal_to_segment_count;

-- NOTE: a table partitioned by a tsquery column cannot be migrated at all, the
-- type of a partitioning column cannot be changed. Such a table has to be
-- dropped by hand before the upgrade, see test/drop_unfixable_objects.sql in
-- the seed scripts.

RESET search_path;
