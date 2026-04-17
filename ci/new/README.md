# Tests

```bash
export IMAGE=gpdb7_ggupgrade:latest
docker build -t "$IMAGE" -f ci/new/Dockerfile .

# test with demo cluster
docker run --rm -v "$(pwd)/logs:/logs" "$IMAGE" bash ggupgrade/ci/new/run_test_in_demo_cluster.bash test command

# test with distributed cluster
bash ci/new/setup_dist_cluster.bash
docker compose -p ggupgrade -f ci/new/docker-compose.yaml exec -u gpadmin -T coordinator bash ggupgrade/ci/new/run_test_in_dist_cluster.bash test command
```
