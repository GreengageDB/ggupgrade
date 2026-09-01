-- check partition indices
--
-- note, that not having
-- sales_1_prt_outlying_dates(trans_id) and
-- sales_1_prt_2(office_id, region)
-- indices is expected, because recreate_partition_indexes
-- scipts concern themselves only with root and child
-- partitions, not the ones in the middle of the
-- hierarchy.

SELECT indrelid::regclass AS table_name,
       indkey,
       indisunique
FROM pg_index pi
JOIN pg_class pc ON pc.oid = pi.indrelid
WHERE pc.relname LIKE 'test_scores%'
    OR pc.relname LIKE 'sales%'
ORDER BY 1, 2;

-- check data
SELECT * FROM test_scores ORDER BY 1, 2;
SELECT * FROM sales ORDER BY 1, 2, 3;

-- insert data
INSERT INTO test_scores VALUES (6, 51);
INSERT INTO test_scores VALUES (7, 61);
INSERT INTO test_scores VALUES (8, 71);
INSERT INTO test_scores VALUES (9, 81);
INSERT INTO test_scores VALUES (10, 91);

INSERT INTO sales VALUES (3, 1, 'usa');
INSERT INTO sales VALUES (3, 2, 'usa');
INSERT INTO sales VALUES (3, 3, 'usa');
INSERT INTO sales VALUES (4, 1, 'zzz');
INSERT INTO sales VALUES (4, 2, 'zzz');
INSERT INTO sales VALUES (4, 3, 'zzz');

-- check data
SELECT * FROM test_scores ORDER BY 1, 2;
SELECT * FROM sales ORDER BY 1, 2, 3;
