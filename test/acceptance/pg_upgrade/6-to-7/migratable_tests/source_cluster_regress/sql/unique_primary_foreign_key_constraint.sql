--------------------------------------------------------------------------------
-- Create and setup migratable objects
--------------------------------------------------------------------------------

CREATE SCHEMA constraints;
SET search_path TO constraints;

CREATE TABLE fk_base_table (a int unique);
CREATE TABLE fk_base_table_2 (c int unique);

CREATE TABLE fk_plain_child (a int REFERENCES fk_base_table(a));
CREATE TABLE fk_part_child (a int REFERENCES fk_base_table(a), int b)
	PARTITION BY RANGE(b) (START(1) END(3) EVERY(1));

CREATE TABLE fk_subpart_child (a int REFERENCES fk_base_table(a),
                               b int,
                               c int REFERENCES fk_base_table_2(c))
	PARTITION BY RANGE(b)
		SUBPARTITION BY RANGE(c)
		SUBPARTITION TEMPLATE (START(1) END(3) EVERY(1))
	(START(1) END(3) EVERY(1));