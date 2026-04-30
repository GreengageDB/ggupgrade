CREATE DATABASE test_database_1;
\c test_database_1

CREATE USER user1;

CREATE TABLE hobbies_r (
	name		text,
	person 		text
);

CREATE TABLE equipment_r (
	name 		text,
	hobby		text
);

CREATE TABLE onek (
	unique1		int4,
	unique2		int4,
	two			int4,
	four		int4,
	ten			int4,
	twenty		int4,
	hundred		int4,
	thousand	int4,
	twothousand	int4,
	fivethous	int4,
	tenthous	int4,
	odd			int4,
	even		int4,
	stringu1	name,
	stringu2	name,
	string4		name
);

CREATE TABLE person (
	name 		text,
	age			int4,
	location 	point
);


CREATE TABLE emp (
	salary 		int4,
	manager 	name
) INHERITS (person) WITH OIDS;


CREATE TABLE student (
	gpa 		float8
) INHERITS (person);


CREATE TABLE stud_emp (
	percent 	int4
) INHERITS (emp, student);


CREATE TABLE city (
	name		name,
	location 	box,
	budget 		city_budget
);

CREATE TABLE dept (
	dname		name,
	mgrname 	text
);

CREATE TABLE slow_emp4000 (
	home_base	 box
);

CREATE TABLE fast_emp4000 (
	home_base	 box
);

CREATE TABLE road (
	name		text,
	thepath 	path
);

CREATE TABLE ihighway () INHERITS (road);

CREATE TABLE shighway (
	surface		text
) INHERITS (road);

CREATE TABLE real_city (
	pop			int4,
	cname		text,
	outline 	path
);

--
-- test the "star" operators a bit more thoroughly -- this time,
-- throw in lots of NULL fields...
--
-- a is the type root
-- b and c inherit from a (one-level single inheritance)
-- d inherits from b and c (two-level multiple inheritance)
-- e inherits from c (two-level single inheritance)
-- f inherits from e (three-level single inheritance)
--
CREATE TABLE a_star (
	class		char,
	a 			int4
);

CREATE TABLE b_star (
	b 			text
) INHERITS (a_star);

CREATE TABLE c_star (
	c 			name
) INHERITS (a_star);

CREATE TABLE d_star (
	d 			float8
) INHERITS (b_star, c_star);

CREATE TABLE e_star (
	e 			int2
) INHERITS (c_star);

CREATE TABLE f_star (
	f 			polygon
) INHERITS (e_star);

COPY person (name, age, location) FROM stdin;
mike	40	(3.1,6.2)
joe	20	(5.5,2.5)
sally	34	(3.8,45.8)
sandra	19	(9.345,09.6)
alex	30	(1.352,8.2)
sue	50	(8.34,7.375)
denise	24	(3.78,87.90)
sarah	88	(8.4,2.3)
teresa	38	(7.7,1.8)
nan	28	(6.35,0.43)
leah	68	(0.6,3.37)
wendy	78	(2.62,03.3)
melissa	28	(3.089,087.23)
joan	18	(9.4,47.04)
mary	08	(3.7,39.20)
jane	58	(1.34,0.44)
liza	38	(9.76,6.90)
jean	28	(8.561,7.3)
jenifer	38	(6.6,23.3)
juanita	58	(4.57,35.8)
susan	78	(6.579,3)
zena	98	(0.35,0)
martie	88	(8.358,.93)
chris	78	(9.78,2)
pat	18	(1.19,0.6)
zola	58	(2.56,4.3)
louise	98	(5.0,8.7)
edna	18	(1.53,3.5)
bertha	88	(2.75,9.4)
sumi	38	(1.15,0.6)
koko	88	(1.7,5.5)
gina	18	(9.82,7.5)
rean	48	(8.5,5.0)
sharon	78	(9.237,8.8)
paula	68	(0.5,0.5)
julie	68	(3.6,7.2)
belinda	38	(8.9,1.7)
karen	48	(8.73,0.0)
carina	58	(4.27,8.8)
diane	18	(5.912,5.3)
esther	98	(5.36,7.6)
trudy	88	(6.01,0.5)
fanny	08	(1.2,0.9)
carmen	78	(3.8,8.2)
lita	25	(1.3,8.7)
pamela	48	(8.21,9.3)
sandy	38	(3.8,0.2)
trisha	88	(1.29,2.2)
uma	78	(9.73,6.4)
velma	68	(8.8,8.9)
\.

INSERT INTO hobbies_r (name, person)
   SELECT 'posthacking', p.name
   FROM person* p
   WHERE p.name = 'mike' or p.name = 'jeff';

INSERT INTO hobbies_r (name, person)
   SELECT 'basketball', p.name
   FROM person p
   WHERE p.name = 'joe' or p.name = 'sally';

INSERT INTO hobbies_r (name) VALUES ('skywalking');

INSERT INTO equipment_r (name, hobby) VALUES ('advil', 'posthacking');

INSERT INTO equipment_r (name, hobby) VALUES ('peet''s coffee', 'posthacking');

INSERT INTO equipment_r (name, hobby) VALUES ('hightops', 'basketball');

INSERT INTO equipment_r (name, hobby) VALUES ('guts', 'skywalking');

INSERT INTO a_star (class, a) VALUES ('a', 1);

INSERT INTO a_star (class, a) VALUES ('a', 2);

INSERT INTO a_star (class) VALUES ('a');

INSERT INTO b_star (class, a, b) VALUES ('b', 3, 'mumble'::text);

INSERT INTO b_star (class, a) VALUES ('b', 4);

INSERT INTO b_star (class, b) VALUES ('b', 'bumble'::text);

INSERT INTO b_star (class) VALUES ('b');

INSERT INTO c_star (class, a, c) VALUES ('c', 5, 'hi mom'::name);

INSERT INTO c_star (class, a) VALUES ('c', 6);

INSERT INTO c_star (class, c) VALUES ('c', 'hi paul'::name);

INSERT INTO c_star (class) VALUES ('c');

INSERT INTO d_star (class, a, b, c, d)
   VALUES ('d', 7, 'grumble'::text, 'hi sunita'::name, '0.0'::float8);

INSERT INTO d_star (class, a, b, c)
   VALUES ('d', 8, 'stumble'::text, 'hi koko'::name);

INSERT INTO d_star (class, a, b, d)
   VALUES ('d', 9, 'rumble'::text, '1.1'::float8);

INSERT INTO d_star (class, a, c, d)
   VALUES ('d', 10, 'hi kristin'::name, '10.01'::float8);

INSERT INTO d_star (class, b, c, d)
   VALUES ('d', 'crumble'::text, 'hi boris'::name, '100.001'::float8);

INSERT INTO d_star (class, a, b)
   VALUES ('d', 11, 'fumble'::text);

INSERT INTO d_star (class, a, c)
   VALUES ('d', 12, 'hi avi'::name);

INSERT INTO d_star (class, a, d)
   VALUES ('d', 13, '1000.0001'::float8);

INSERT INTO d_star (class, b, c)
   VALUES ('d', 'tumble'::text, 'hi andrew'::name);

INSERT INTO d_star (class, b, d)
   VALUES ('d', 'humble'::text, '10000.00001'::float8);

INSERT INTO d_star (class, c, d)
   VALUES ('d', 'hi ginger'::name, '100000.000001'::float8);

INSERT INTO d_star (class, a) VALUES ('d', 14);

INSERT INTO d_star (class, b) VALUES ('d', 'jumble'::text);

INSERT INTO d_star (class, c) VALUES ('d', 'hi jolly'::name);

INSERT INTO d_star (class, d) VALUES ('d', '1000000.0000001'::float8);

INSERT INTO d_star (class) VALUES ('d');

INSERT INTO e_star (class, a, c, e)
   VALUES ('e', 15, 'hi carol'::name, '-1'::int2);

INSERT INTO e_star (class, a, c)
   VALUES ('e', 16, 'hi bob'::name);

INSERT INTO e_star (class, a, e)
   VALUES ('e', 17, '-2'::int2);

INSERT INTO e_star (class, c, e)
   VALUES ('e', 'hi michelle'::name, '-3'::int2);

INSERT INTO e_star (class, a)
   VALUES ('e', 18);

INSERT INTO e_star (class, c)
   VALUES ('e', 'hi elisa'::name);

INSERT INTO e_star (class, e)
   VALUES ('e', '-4'::int2);

INSERT INTO f_star (class, a, c, e, f)
   VALUES ('f', 19, 'hi claire'::name, '-5'::int2, '(1,3),(2,4)'::polygon);

INSERT INTO f_star (class, a, c, e)
   VALUES ('f', 20, 'hi mike'::name, '-6'::int2);

INSERT INTO f_star (class, a, c, f)
   VALUES ('f', 21, 'hi marcel'::name, '(11,44),(22,55),(33,66)'::polygon);

INSERT INTO f_star (class, a, e, f)
   VALUES ('f', 22, '-7'::int2, '(111,555),(222,666),(333,777),(444,888)'::polygon);

INSERT INTO f_star (class, c, e, f)
   VALUES ('f', 'hi keith'::name, '-8'::int2,
	   '(1111,3333),(2222,4444)'::polygon);

INSERT INTO f_star (class, a, c)
   VALUES ('f', 24, 'hi marc'::name);

INSERT INTO f_star (class, a, e)
   VALUES ('f', 25, '-9'::int2);

INSERT INTO f_star (class, a, f)
   VALUES ('f', 26, '(11111,33333),(22222,44444)'::polygon);

INSERT INTO f_star (class, c, e)
   VALUES ('f', 'hi allison'::name, '-10'::int2);

INSERT INTO f_star (class, c, f)
   VALUES ('f', 'hi jeff'::name,
           '(111111,333333),(222222,444444)'::polygon);

INSERT INTO f_star (class, e, f)
   VALUES ('f', '-11'::int2, '(1111111,3333333),(2222222,4444444)'::polygon);

INSERT INTO f_star (class, a) VALUES ('f', 27);

INSERT INTO f_star (class, c) VALUES ('f', 'hi carl'::name);

INSERT INTO f_star (class, e) VALUES ('f', '-12'::int2);

INSERT INTO f_star (class, f)
   VALUES ('f', '(11111111,33333333),(22222222,44444444)'::polygon);

INSERT INTO f_star (class) VALUES ('f');


CREATE DATABASE test_database_2;
\c test_database_2

CREATE TABLE aoro(a int, b int) WITH (appendonly=true, orientation=row) DISTRIBUTED BY (a);
CREATE TABLE aoco(a int, b int) WITH (appendonly=true, orientation=column) DISTRIBUTED BY (a);

INSERT INTO aoro SELECT i, i * 100 FROM generate_series(1, 10000) i;
INSERT INTO aoco SELECT * FROM aoro;
CREATE FUNCTION test_func(int) RETURNS bool LANGUAGE 'sql' AS 'SELECT $1 > 50';