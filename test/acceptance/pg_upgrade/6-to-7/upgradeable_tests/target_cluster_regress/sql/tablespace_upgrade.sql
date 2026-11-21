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

select count(*) from ts1_test;
select count(*) from ts2_test;

drop table ts1_test;
drop table ts2_test;
drop tablespace test_tablespace1;
drop tablespace test_tablespace2;
! rm -rf /tmp/tablespace1;
! rm -rf /tmp/tablespace2;
! rm -rf /tmp/tablespace3;
