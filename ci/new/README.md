# How to run tests

## Unit and Integration tests

### Unit
```bash
make unit
```
### Integration
```bash
# The ggupgrade binary should be available in the PATH before running integration tests
make
make install
make integration
```

## Acceptance and pg-upgrade-tests tests
Both of these tests require a running cluster. You can use a Docker container for that:
```bash
docker build -t gpdb7_ggupgrade:latest -f ci/new/Dockerfile .
```

To run them on a demo cluster, you can use the following script:
```bash
bash ci/new/run_test_in_demo_cluster.bash make acceptance or pg-upgrade-tests
```
