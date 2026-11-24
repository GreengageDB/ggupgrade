-- start_ignore
drop table if exists ts1_test;
drop table if exists ts2_test;
-- end_ignore
! mkdir /tmp/tablespace1;
! mkdir /tmp/tablespace2;
! mkdir /tmp/tablespace3;
create tablespace test_tablespace1 location '/tmp/tablespace1';
create tablespace test_tablespace2 location '/tmp/tablespace2' with (content1 = '/tmp/tablespace3');

select
    -1 as gp_segment_id,
    spcname,
    pg_tablespace_location(oid) as location
    from pg_tablespace
union all
select
    gp_segment_id,
    spcname,
    pg_tablespace_location(oid) as location
    from gp_dist_random('pg_tablespace');

create table ts1_test (a int,  b int) tablespace test_tablespace1;
insert into ts1_test select a, a from generate_series(1, 10000)a;

create table ts2_test (a int, b int) tablespace test_tablespace2;
insert into ts2_test select a, a from generate_series(1, 10000)a;
