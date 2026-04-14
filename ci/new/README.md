# Tests

Tests supposed to run in Docker container. Docker Image can be built with next command:
```bash
docker build -t gpdb7_ggupgrade:latest -f ci/new/Dockerfile .
```

To run test with demo cluster:
```bash
export IMAGE=gpdb7_ggupgrade:latest
bash ci/new/run_test_in_demo_cluster.bash test command
```

To run test in dist cluster:
```bash
export IMAGE=gpdb7_ggupgrade:latest
bash ci/new/run_test_in_dist_cluster.bash test command
```
