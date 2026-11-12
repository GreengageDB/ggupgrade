select spcname from pg_tablespace;

select count(*) from ts_test;

drop table ts_test;
drop tablespace test_tablespace;
! rm -rf /tmp/tablespace;
