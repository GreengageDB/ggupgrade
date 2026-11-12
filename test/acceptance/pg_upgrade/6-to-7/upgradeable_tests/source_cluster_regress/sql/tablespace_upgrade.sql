-- start_ignore
drop table if exists t1;
-- end_ignore
! mkdir /tmp/tablespace;
create tablespace test_tablespace location '/tmp/tablespace';
create table ts_test (a int,  b int) tablespace test_tablespace;
insert into ts_test select a, a from generate_series(1, 10000)a;
