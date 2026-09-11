--------------------------------------------------------------------------------
-- Create and setup migratable objects
--------------------------------------------------------------------------------
CREATE SCHEMA constraints;
SET search_path TO constraints;

-- foreign key constraints
CREATE TABLE table_with_fk_base_table (a int unique);
CREATE TABLE table_with_fk_pt_with_index (
    a int REFERENCES table_with_fk_base_table(a),
    b int,
    c int,
    d int
) PARTITION BY RANGE(b)
(
    PARTITION pt1 START(1),
    PARTITION pt2 START(2) END(3),
    PARTITION pt3 START(3) END(4)
);

CREATE INDEX table_with_fk_pt_idx_c on table_with_fk_pt_with_index(c);
CREATE INDEX table_with_fk_pt_idx_c_bitmap on table_with_fk_pt_with_index using bitmap(c);

CREATE INDEX table_with_fk_pt_idx_b_prt_2 on table_with_fk_pt_with_index_1_prt_pt2(b);
CREATE INDEX table_with_fk_pt_idx_b_prt_2_bitmap on table_with_fk_pt_with_index_1_prt_pt2 using bitmap(b);

CREATE INDEX table_with_fk_pt_idx_c_prt_2 on table_with_fk_pt_with_index_1_prt_pt2(c);
CREATE INDEX table_with_fk_pt_idx_c_prt_2_bitmap on table_with_fk_pt_with_index_1_prt_pt2 using bitmap(c);

INSERT INTO table_with_fk_pt_with_index VALUES (1, 1, 1, 1);
INSERT INTO table_with_fk_pt_with_index VALUES (2, 2, 2, 2);

CREATE TABLE table_with_fk_plain_child (a int REFERENCES table_with_fk_base_table(a));

CREATE TABLE table_with_fk_ao_child (a int REFERENCES table_with_fk_base_table(a), b int) WITH(appendonly=true);

-- check foreign key constraints
SELECT nspname, relname, conname
FROM pg_constraint cc
JOIN pg_class c ON c.oid = cc.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE cc.contype = 'f'
AND c.relname LIKE 'table_with_fk_%';

-- check indexes
SELECT c.relname AS index_name
FROM pg_index i
JOIN pg_class c ON i.indexrelid = c.oid
JOIN pg_class t ON i.indrelid = t.oid
AND t.relname LIKE 'table_with_fk_pt_%';

-- check data
SELECT * FROM table_with_fk_pt_with_index ORDER BY 1, 2, 3, 4;



-- unique constraints
CREATE TABLE table_with_unique_constraint (
    author int,
    title int,
    CONSTRAINT table_with_unique_constraint_uniq_au_ti UNIQUE (author, title)
) DISTRIBUTED BY (author);

ALTER TABLE table_with_unique_constraint ADD PRIMARY KEY (author, title);
INSERT INTO table_with_unique_constraint VALUES (1, 1);
INSERT INTO table_with_unique_constraint VALUES (2, 2);

CREATE TABLE table_with_unique_constraint_p (
    author int,
    title int,
    CONSTRAINT unique_constraint_p_uniq_au_ti UNIQUE (author, title)
) PARTITION BY RANGE(title) (START(1) END(4) EVERY(1));

ALTER TABLE table_with_unique_constraint_p ADD PRIMARY KEY (author, title);
INSERT INTO table_with_unique_constraint_p VALUES (1, 1);
INSERT INTO table_with_unique_constraint_p VALUES (2, 2);

-- check unique constraints
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
    AND contype IN ('u', 'p', 'x')
JOIN non_child_partitions c ON objid = c.oid
    AND relkind = 'i'
JOIN non_child_partitions cc ON cc.oid = con.conrelid
JOIN pg_namespace n ON (n.oid = cc.relnamespace)
WHERE cc.relname LIKE 'table_with_unique_constraint%'
ORDER BY 1, 2, 3;

-- check data
SELECT * FROM table_with_unique_constraint ORDER BY 1, 2;
SELECT * FROM table_with_unique_constraint_p ORDER BY 1, 2;



-- primary constraints
CREATE TABLE table_with_primary_constraint (
    author int,
    title int,
    CONSTRAINT table_with_primary_constraint_au_ti PRIMARY KEY (author, title)
) DISTRIBUTED BY (author);

ALTER TABLE table_with_primary_constraint ADD UNIQUE (author, title);
INSERT INTO table_with_primary_constraint VALUES (1, 1);
INSERT INTO table_with_primary_constraint VALUES (2, 2);

CREATE TABLE table_with_primary_constraint_p (
    author int,
    title int,
    CONSTRAINT primary_constraint_p_au_ti PRIMARY KEY (author, title)
) PARTITION BY RANGE(title) (START(1) END(4) EVERY(1));

ALTER TABLE table_with_primary_constraint_p ADD UNIQUE (author, title);
INSERT INTO table_with_primary_constraint_p VALUES (1, 1);
INSERT INTO table_with_primary_constraint_p VALUES (2, 2);

-- check primary unique constraints
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
    AND contype IN ('u', 'p', 'x')
JOIN non_child_partitions c ON objid = c.oid
    AND relkind = 'i'
JOIN non_child_partitions cc ON cc.oid = con.conrelid
JOIN pg_namespace n ON (n.oid = cc.relnamespace)
WHERE cc.relname LIKE 'table_with_primary_constraint%'
ORDER BY 1, 2, 3;

-- check data
SELECT * FROM table_with_primary_constraint ORDER BY 1, 2;
SELECT * FROM table_with_primary_constraint_p ORDER BY 1, 2;
