CREATE OR REPLACE FUNCTION __ggupgrade_tmp_generator.raise_partition_error(schemaname TEXT, relname TEXT) RETURNS TEXT
LANGUAGE plpgsql AS
$$
BEGIN
    RAISE EXCEPTION 'Relation %.% cannot be access neither by name, nor by RANK.', schemaname, relname;
END
$$;

CREATE OR REPLACE FUNCTION __ggupgrade_tmp_generator.partition_name(schemaname NAME, tablename NAME, name NAME, rank BIGINT)
RETURNS TEXT
LANGUAGE plpgsql AS
$$
BEGIN
    RETURN CASE
        WHEN COALESCE(name, '') != '' THEN quote_ident(name)
        WHEN rank IS NOT NULL THEN FORMAT('FOR(RANK(%s))', rank)
        ELSE __ggupgrade_tmp_generator.raise_partition_error(schemaname, tablename)
    END;
END
$$;

WITH RECURSIVE affected_partitions AS (
    SELECT rp.oid as parrelid, cp1.parparentrule, cp1.childnamespace, cp1.childrelname, cp1.parname, rp.parrelname, p3.schemaname, p3.partitionname, p3.partitionrank, p3.partitionposition, p3.parentpartitiontablename, p3.partitiontablename, p3.partitionschemaname, cp1.childrelowner, __ggupgrade_tmp_generator.partition_name(p3.partitionschemaname, p3.partitiontablename, p3.partitionname, p3.partitionrank) parid
        FROM (
                SELECT p.parrelid, rule.parparentrule, rule.parchildrelid, rule.parname, n.nspname AS childnamespace, c.relname AS childrelname, c.relnatts AS childnatts,
                       sum(CASE WHEN a.attisdropped THEN 1 ELSE 0 END) AS childnumattisdropped, r.rolname AS childrelowner
                FROM pg_catalog.pg_partition p
                    JOIN pg_catalog.pg_partition_rule rule ON p.oid=rule.paroid AND NOT p.paristemplate
                    JOIN pg_catalog.pg_class c ON rule.parchildrelid = c.oid AND NOT c.relhassubclass
                    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
                    JOIN pg_catalog.pg_attribute a ON rule.parchildrelid = a.attrelid AND a.attnum > 0
                    JOIN pg_roles r ON c.relowner = r.oid
                GROUP BY p.parrelid, rule.parparentrule, rule.parchildrelid, rule.parname, n.nspname, c.relname, c.relnatts, r.rolname
            ) cp1
            JOIN (
                SELECT p.parrelid, min(c.relnatts) AS minchildnatts, max(c.relnatts) AS maxchildnatts
                FROM pg_catalog.pg_partition p
                    JOIN pg_catalog.pg_partition_rule rule ON p.oid=rule.paroid AND NOT p.paristemplate
                    JOIN pg_catalog.pg_class c ON rule.parchildrelid = c.oid AND NOT c.relhassubclass
                GROUP BY p.parrelid
            ) cp2 ON cp2.parrelid = cp1.parrelid
            JOIN (
                SELECT c.oid, n.nspname AS parnamespace, c.relname AS parrelname, c.relnatts AS parnatts,
                       sum(CASE WHEN a.attisdropped THEN 1 ELSE 0 END) AS parnumattisdropped
                FROM pg_catalog.pg_partition p
                    JOIN pg_catalog.pg_class c ON p.parrelid = c.oid AND NOT p.paristemplate AND p.parlevel = 0
                    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
                    JOIN pg_catalog.pg_attribute a ON c.oid = a.attrelid AND a.attnum > 0
                GROUP BY c.oid, n.nspname, c.relname, c.relnatts
            ) rp ON rp.oid = cp1.parrelid
            JOIN pg_partitions p3 ON cp1.childrelname = p3.partitiontablename AND cp1.childnamespace = p3.partitionschemaname
        WHERE NOT (rp.parnumattisdropped = 0 AND rp.parnatts = cp1.childnatts) AND
              NOT (rp.parnumattisdropped > 0 AND cp2.minchildnatts = cp2.maxchildnatts AND
                   (rp.parnatts = cp1.childnatts OR cp1.childnumattisdropped = 0)) AND
              NOT (rp.parnumattisdropped > 0 AND cp2.minchildnatts != cp2.maxchildnatts AND
                   cp2.minchildnatts < rp.parnatts AND cp1.childnumattisdropped = 0) AND
              NOT (rp.parnumattisdropped > 0 AND cp2.minchildnatts != cp2.maxchildnatts AND
                   cp2.minchildnatts >= rp.parnatts)
),
incomplete_paths AS (
    SELECT affected_partitions.parrelid,
           affected_partitions.parparentrule,
           affected_partitions.parid,
           NULL::TEXT[] AS path,
           affected_partitions.partitiontablename,
           affected_partitions.partitionschemaname,
           affected_partitions.childrelowner
    FROM affected_partitions
    UNION
    SELECT h.parrelid,
           parent.parparentrule,
           h.parid,
           array_prepend(
                __ggupgrade_tmp_generator.partition_name(
                    parts.partitionschemaname,
                    parts.partitiontablename,
                    parent.parname,
                    parts.partitionrank
                ),
                h.path
           ) AS path,
           h.partitiontablename,
           h.partitionschemaname,
           h.childrelowner
    FROM incomplete_paths h
        JOIN pg_partition_rule parent ON h.parparentrule = parent.oid
        JOIN pg_class c ON c.oid = parent.parchildrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_partitions parts ON c.relname = parts.partitiontablename AND n.nspname = parts.partitionschemaname
),
commands AS (
    SELECT FORMAT(
        '
        CREATE TABLE __ggupgrade_tmp_executor.scratch_table (LIKE %I.%I INCLUDING CONSTRAINTS INCLUDING DEFAULTS);
        ALTER TABLE __ggupgrade_tmp_executor.scratch_table OWNER TO %I;
        INSERT INTO  __ggupgrade_tmp_executor.scratch_table SELECT * FROM %I.%I;
        ALTER TABLE %I.%I%s EXCHANGE PARTITION %s WITH TABLE __ggupgrade_tmp_executor.scratch_table;
        DROP TABLE __ggupgrade_tmp_executor.scratch_table;
        ',
        partitionschemaname,
        partitiontablename,
        childrelowner,
        partitionschemaname,
        partitiontablename,
        nspname,
        relname,
        (SELECT CASE path IS NULL
            WHEN true THEN ''
            ELSE ' ALTER PARTITION ' || array_to_string(incomplete_paths.path, ' ALTER PARTITION ')
        END),
        parid
    ) command
    FROM incomplete_paths
        JOIN pg_class c ON c.oid = incomplete_paths.parrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE parparentrule = 0
)
SELECT FORMAT (
    '
        SET gp_enable_exchange_default_partition = on;
        SET optimizer_enable_ctas = off;
        CREATE SCHEMA __ggupgrade_tmp_executor;
        DROP TABLE IF EXISTS __ggupgrade_tmp_executor.scratch_table;
        %s
        DROP SCHEMA __ggupgrade_tmp_executor CASCADE;
        RESET gp_enable_exchange_default_partition;
        RESET optimizer_enable_ctas;
    ',
    string_agg(command, '')
)
FROM commands;
