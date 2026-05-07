# Tests

```bash
export IMAGE=gpdb7_ggupgrade:latest
docker build -t "$IMAGE" -f ci/new/Dockerfile .

# test with demo cluster
docker run --rm -v "$(pwd)/logs:/logs" "$IMAGE" bash ggupgrade/ci/new/run_test_in_demo_cluster.bash test command

# test with distributed cluster
bash ci/new/setup_dist_cluster.bash
docker compose -p ggupgrade -f ./ci/new/docker-compose.yaml exec \
               -u gpadmin -T coordinator \
               env WITH_MIRRORS='true' WITH_STANDBY='true' \
               ggupgrade/ci/new/run_test_in_dist_cluster.bash \
               test command
```

## Acceptance tests

```bash
# test with demo cluster
docker run --rm -v "$(pwd)/logs:/logs" "$IMAGE" bash ggupgrade/ci/new/run_test_in_demo_cluster.bash make acceptance

# test with distributed cluster
bash ci/new/setup_dist_cluster.bash
docker compose -p ggupgrade -f ./ci/new/docker-compose.yaml exec \
               -u gpadmin -T coordinator \
               env WITH_MIRRORS='true' WITH_STANDBY='true' \
               ggupgrade/ci/new/run_test_in_dist_cluster.bash \
               go test --cover -count=1 -timeout 30m -v -run '^TestRevert$' ./test/acceptance/ggupgrade
```

## pg_upgrade tests

```bash
# test with demo cluster
docker run --rm -v "$(pwd)/logs:/logs" "$IMAGE" bash ggupgrade/ci/new/run_test_in_demo_cluster.bash make pg-upgrade-tests
```