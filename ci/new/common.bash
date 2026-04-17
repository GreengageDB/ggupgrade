function collect_logs {
    params=(
      "./ d gpAdminLogs"
      "$1 d log"
      "$1 d pg_log"
      "ggupgrade/test/acceptance/pg_upgrade/6-to-7/ d results"
    )

    shift

    for param in "${params[@]}"; do
      read -r path type name <<< "$param"
      $@ "find \"$path\" -name \"$name\" -type \"$type\" -exec tar -rf \"/logs/$name.tar\" {} \;" || true
    done

    cp ggupgrade/test/acceptance/pg_upgrade/6-to-7/non_upgradeable_tests/regression.diffs /logs/regression_non_upgradeable.diffs || true
    cp ggupgrade/test/acceptance/pg_upgrade/6-to-7/upgradeable_tests/source_cluster_regress/regression.diffs /logs/regression_upgradeable.diffs || true
}

export PATH=$PATH:/opt/go/bin:~/go/bin
