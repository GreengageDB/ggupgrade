-- Tables partitioned by multiple columns
\c regression
drop table bfv_partition.t26002_t1;
drop table bfv_partition.t26002_t1;
drop table dpe_malp.malp;
drop table partition_pruning.pt_complex;
drop table partition_pruning.pt_complex;
drop table public.dml_ao_pt_p;
drop table public.dml_co_pt_p;
drop table public.dml_heap_pt_p;
drop table public.equal_operator_not_in_search_path_table_multi_key;
drop table public.mpp18162c;
drop table public.mpp18162a;
drop table public.mpp18162b;
drop table public.mpp18162f;
drop table public.mpp18179;
drop table public.mpp18162d;
drop table public.mpp18162e;
drop table public.mpp5878;
drop table public.mpp5878a;

\c contrib_regression_gplibpq
drop function pg_ctl(datadir text, command text, port integer);


-- plpythonu dependent functions
\c isolation2test
drop function public.exec_cmd_on_segments(cmd text);
drop function public.pg_controldata_redo_lsn(datadir text);
drop function public.pg_ctl(datadir text, command text, command_mode text);
drop function public.pg_ctl_start(datadir text, port integer);

\c mapred_regression
drop function public.execute(cmd text);
drop function public.mapreduce(file text);
drop function public.mapreduce(file text, keys text);
drop function public.python_path();
drop function public.python_version();

\c regression
drop function bfv_catalog.count_operator(query text, operator text);
drop function bfv_legacy.count_operator(explain_query text, op_name text);
drop function bfv_legacy.nonzero_width(explain_query text);
drop function memconsumption.has_account_type(query text, search_text text);
drop function memconsumption.sum_owner_consumption(query text, owner text);
drop function partition_pruning.get_selected_parts(explain_query text);
drop function public.change_file_permission_readonly(path text);
drop function public.check_workfile_compressed(explain_query text, is_comp_buff_limit boolean);
drop function public.count_operator(query text, operator text);
drop function public.db_dirs(dboid oid);
drop function public.dml_fn2(x integer);
drop function public.get_temp_file_num();
drop function public.gp_tablespace_watch_log(dbid integer, message text);
drop function public.gp_tablespace_watch_match(dbid integer, name text, patstr text);
drop function public.gp_tablespace_watch_start(dbid integer, name text, location text);
drop function public.gp_tablespace_watch_stop();
drop function public.insert_correct();
drop function public.isspilling(explain_query text);
drop function public.list_tablespace_dbid_dirs(expected_number_of_tablespaces integer, tablespace_location_directory text);
drop function public.remove_tablespace_location_directory(tablespace_location_dir text);
drop function public.run_all_in_one();
drop function public.save_keepalives_data(result_table text);
drop function public.setup_tablespace_location_dir_for_test(tablespace_location_dir text);
drop function public.stat_table_segfile_size(datname text, tabname text);
drop function public.test_bigint_python();
drop function qp_misc_rio.func_array_argument_plpythonu(arg double precision[]);
drop function qp_misc_rio.func_plpythonu(n integer);
drop function qp_misc_rio.func_plpythonu2(x integer);
drop function qp_misc_rio.t18_pytest();
drop function qp_query_execution.qx_count_operator(query text, planner_operator text, optimizer_operator text);
drop function qp_with_functional_inlining.cte_func3();
drop function qp_with_functional_noinlining.cte_func3();
drop function sort_schema.has_sortmethod(explain_analyze_query text);


-- plpython itself
\c contrib_regression_gplibpq
drop language plpythonu;

\c contrib_regression_heap_checksum_helper
drop language plpythonu;

\c gppc_regress
drop language plpythonu;

\c isolation2test
drop language plpythonu;

\c mapred_regression
drop language plpythonu;

\c regression;
drop extension plpythonu;

-- HACK: I have not found a way to escape the name of this db. Even $$ syntax doesn't seem to work  with \c command
\! psql "funny copy\"db'with\\\\quotes" -c "drop language plpythonu";


-- Views with removed functions
\c regression;
drop view mpp7164.partagain;
drop view mpp7164.partrank;


-- Disallowed OPERATOR =>
\c regression
drop operator => (int8, none);


-- Views with changed functions
\c mapred_regression
drop view public.env;


-- Removed "abstime" data type
\c regression
drop table gpdist_legacy_opclasses.all_legacy_types;


-- Tables with OIDS
\c regression
alter table bfv_dml.tabwithoids set without oids;
alter table public.emp set without oids;
alter table public.stud_emp set without oids;
alter table public.tenk1 set without oids;
alter table public.tt7 set without oids;
alter table qp_dml_oids.dml_ao set without oids;
alter table qp_dml_oids.dml_heap_check_r set without oids;
alter table qp_dml_oids.dml_heap_p set without oids;
alter table qp_dml_oids.dml_heap_r set without oids;
alter table qp_dml_oids.dml_heap_with_oids set without oids;
alter table sort_schema.gpsort_alltypes set without oids;


-- Invalid "unknown" columns
\c regression
drop table public.aocs_unknown;
drop materialized view public.mv_unspecified_types;
drop table public.test_issue_12936;


-- Unavailable extensions
\c contrib_regression
drop extension gp_subtransaction_overflow;


-- NOMERGE: Start of the workarouds. These are legit bugs, but right now I want to see more of them

-- Missing guc value. gp_default_storage_options is depricated as of Greengage 7: https://github.com/GreengageDB/greengage/commit/19cd1cf4b68faff2e29bc2fa884c480e4644cdb4
ALTER DATABASE dsp1 RESET gp_default_storage_options;
ALTER DATABASE dsp2 RESET gp_default_storage_options;

-- We are hitting an assertions because of the 