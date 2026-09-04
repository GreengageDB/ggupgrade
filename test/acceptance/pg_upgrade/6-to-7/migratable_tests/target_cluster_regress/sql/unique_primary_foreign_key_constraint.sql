SET search_path TO constraints;

-- A table to hold interesting relations, to make sure that 
-- we don't duplicate code, as we can't detect partitions
-- in a cross-version way
CREATE TABLE interesting_relations AS (
    SELECT oid FROM pg_class
    WHERE relname IN (
        'fk_base_table',
        'fk_pt_with_index',
        'table_with_unique_constraint',
        'table_with_unique_constraint_p',
        'table_with_primary_constraint',
        'table_with_primary_constraint_p'
    )
);


SELECT nspname, relname, conname
FROM pg_constraint cc
JOIN pg_class c ON c.oid = cc.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE cc.contype = 'f' AND EXISTS (
    SELECT 1 FROM interesting_relations rels
    WHERE c.oid = rels.oid
);

-- check indexes
SELECT c.relname AS index_name
FROM pg_index i
JOIN pg_class c ON i.indexrelid = c.oid
JOIN pg_class t ON i.indrelid = t.oid
AND t.relname LIKE 'fk_pt_%';

-- check data
SELECT * FROM fk_pt_with_index ORDER BY 1, 2, 3, 4;

-- insert data and exercise constraint
INSERT INTO fk_pt_with_index VALUES (3, 3, 3, 3);
INSERT INTO fk_pt_with_index VALUES (3, 3, 3, 3);

-- check data
SELECT * FROM fk_pt_with_index ORDER BY 1, 2, 3, 4;


-- check unique constraints
SELECT n.nspname, cc.relname, conname
FROM pg_constraint con
JOIN pg_depend dep
    ON (refclassid, classid, objsubid) = ('pg_constraint'::regclass, 'pg_class'::regclass, 0)
    AND refobjid = con.oid
    AND deptype = 'i'
    AND contype IN ('u', 'p', 'x')
JOIN pg_class c ON objid = c.oid
    AND c.relkind = 'i'
JOIN pg_class cc ON cc.oid = con.conrelid
JOIN pg_namespace n ON (n.oid = cc.relnamespace)
WHERE cc.relname LIKE 'table_with_unique_constraint%' AND EXISTS (
    SELECT 1 
    FROM interesting_relations rels
    WHERE cc.oid = rels.oid
)
ORDER BY 1, 2, 3;

-- check data
SELECT * FROM table_with_unique_constraint ORDER BY 1, 2;
SELECT * FROM table_with_unique_constraint_p ORDER BY 1, 2;

-- insert data and exercise constraint
INSERT INTO table_with_unique_constraint VALUES (3, 3);
INSERT INTO table_with_unique_constraint VALUES (3, 3);
INSERT INTO table_with_unique_constraint_p VALUES (3, 3);
INSERT INTO table_with_unique_constraint_p VALUES (3, 3);

-- check data
SELECT * FROM table_with_unique_constraint ORDER BY 1, 2;
SELECT * FROM table_with_unique_constraint_p ORDER BY 1, 2;



-- check primary unique constraints
SELECT n.nspname, cc.relname, conname
FROM pg_constraint con
JOIN pg_depend dep
    ON (refclassid, classid, objsubid) = ('pg_constraint'::regclass, 'pg_class'::regclass, 0)
    AND refobjid = con.oid
    AND deptype = 'i'
    AND contype IN ('u', 'p', 'x')
JOIN pg_class c ON objid = c.oid
    AND c.relkind = 'i'
JOIN pg_class cc ON cc.oid = con.conrelid
JOIN pg_namespace n ON (n.oid = cc.relnamespace)
WHERE cc.relname LIKE 'table_with_primary_constraint%' AND EXISTS (
    SELECT 1 
    FROM interesting_relations rels
    WHERE cc.oid = rels.oid
)
ORDER BY 1, 2, 3;

-- check data
SELECT * FROM table_with_primary_constraint ORDER BY 1, 2;
SELECT * FROM table_with_primary_constraint_p ORDER BY 1, 2;

-- insert data and exercise constraint
INSERT INTO table_with_primary_constraint VALUES (3, 3);
INSERT INTO table_with_primary_constraint VALUES (3, 3);
INSERT INTO table_with_primary_constraint_p VALUES (3, 3);
INSERT INTO table_with_primary_constraint_p VALUES (3, 3);

-- check data
SELECT * FROM table_with_primary_constraint ORDER BY 1, 2;
SELECT * FROM table_with_primary_constraint_p ORDER BY 1, 2;
