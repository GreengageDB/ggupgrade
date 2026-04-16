# Tests

Tests supposed to run in Docker container. Docker Image can be built with next command:
```bash
docker build -t ggdb7_ggupgrade:latest -f ci/new/Dockerfile .
```

To run test with demo cluster:
```bash
docker run --rm -d --init --name ggupgrade_test -v "$(pwd)/logs:/logs" ggdb7_ggupgrade:latest sleep infinity
docker exec ggupgrade_test bash /home/gpadmin/ggupgrade/ci/new/run_test_in_demo_cluster.bash test command
```

To run test in dist cluster:
```bash
export IMAGE=ggdb7_ggupgrade:latest
bash ci/new/setup_dist_cluster.bash
docker compose -p ggupgrade -f ci/new/docker-compose.yaml exec -u gpadmin -T coordinator \
 bash /home/gpadmin/ggupgrade/ci/new/run_test_in_dist_cluster.bash test command
```
