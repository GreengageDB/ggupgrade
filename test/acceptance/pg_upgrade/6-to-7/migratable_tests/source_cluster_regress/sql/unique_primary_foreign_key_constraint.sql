--------------------------------------------------------------------------------
-- Create and setup migratable objects
--------------------------------------------------------------------------------

CREATE SCHEMA constraints;
SET search_path TO constraints;

CREATE TABLE fk_base_table (a int unique);
CREATE TABLE fk_base_table_2 (c int unique);

CREATE TABLE fk_plain_child (a int REFERENCES fk_base_table(a));
CREATE TABLE fk_part_child (a int REFERENCES fk_base_table(a), b int)
	PARTITION BY RANGE(b) (START(1) END(3) EVERY(1));

CREATE TABLE fk_subpart_child (a int REFERENCES fk_base_table(a),
                               b int,
                               c int REFERENCES fk_base_table_2(c))
	PARTITION BY RANGE(b)
		SUBPARTITION BY RANGE(c)
		SUBPARTITION TEMPLATE (START(1) END(3) EVERY(1))
	(START(1) END(3) EVERY(1));

WITH non_child_partitions AS (
    SELECT oid, *
    FROM pg_class
    WHERE oid NOT IN (
        SELECT DISTINCT parchildrelid
        FROM pg_partition_rule
    )
)
SELECT n.nspname, cc.relname, conname
FROM pg_constraint con
JOIN pg_depend dep
    ON (refclassid, classid, objsubid) = ('pg_constraint'::regclass, 'pg_class'::regclass, 0)
    AND refobjid = con.oid
    AND deptype = 'i'
    AND contype IN ('f')
JOIN non_child_partitions c ON objid = c.oid
    AND relkind = 'i'
JOIN non_child_partitions cc ON cc.oid = con.conrelid
JOIN pg_namespace n ON (n.oid = cc.relnamespace)
WHERE cc.relname LIKE 'fk_%'
ORDER BY 1, 2, 3;
