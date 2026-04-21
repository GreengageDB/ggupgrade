# ggupgrade

ggupgrade runs [pg_upgrade](https://www.postgresql.org/docs/current/static/pgupgrade.html)
across all segments to upgrade a [Greengage cluster](https://github.com/GreengageDB/greengage)
across major versions. For further details read the Greengage Database Upgrade documentation.
We warmly welcome any feedback and
[contributions](https://github.com/GreengageDB/ggupgrade/blob/main/CONTRIBUTING.md).

**Purpose:**

Greengage has multiple ways of upgrading including backup & restore and gpcopy.
These methods usually require additional diskspace for the required copy and 
significant downtime. ggupgrade can do fast in-place upgrades without the need 
for additional hardware, disk space, and with less downtime. 

Creating an easy upgrade path enables users to quickly and confidently upgrade. 
This enables Greengage to have faster release cycles with faster user feedback. 
Most importantly it allows Greengage to reduce its reliance on supporting legacy 
versions.

**Supported Versions:**

| Source Cluster | Target Cluster
| --- | ---
| 5 | 6
| 6 | 7 (future work)
 
**Architecture:**

ggupgrade consists of three processes that communicate using gRPC and protocol buffers:
- CLI
  - Runs on the coordinator host
  - Consists of a gRPC client
- Hub
  - Runs on the coordinator host
  - Upgrades the coordinator
  - Coordinates the agent processes
  - Consists of a gRPC client and server 
- Agents
  - Run on all segment hosts
  - Upgrade the standby, primary, and mirror segments
  - Execute commands received from the hub
  - Consist of a gRPC server
 
```
       CLI                     Hub                     Agent
      ------                  ------                  -------
  gRPC client    <-->      gRPC server
                                ^
                                |
                                V
                           gRPC client     <-->      gRPC server
```

**Steps:**

Running ggupgrade consists of several steps (ie: commands):
- ggupgrade initialize
  - The source cluster can still be running. No downtime.
  - Substeps include creating the ggupgrade state directory, starting the hub and agents, creating the target cluster, 
    and running pre-upgrade checks.
- ggupgrade execute
  - This step will stop the source cluster. Downtime is needed.
  - Substeps include upgrading the coordinator, copying the coordinator catalog to the segments, and upgrading the primaries.
  - The coordinator contains only catalog information and no data, and is used to upgrade the catalog of the primaries. 
    That is, after the target cluster coordinator is upgraded it's copied to each of the primary data directories on 
    the target cluster to upgrade their catalog. Next, the target cluster primaries data is upgraded using pg_upgrade.
- ggupgrade finalize
  - After finalizing the upgrade cannot be reverted.
  - Substeps include updating the data directories and coordinator catalog, and upgrading the standby and mirrors.

Optional steps (ie: commands):
- ggupgrade revert
  - To restore the cluster to the state before the upgrade.
  - Can be run after initialize or execute, but *not* finalize.
  - Substeps include deleting the target cluster, archiving the ggupgrade log 
    directory, and restoring the source cluster.
  - Reverting in copy mode consists of simply removing the target cluster.
    However, due to a GPDB 5X bug gprecoverseg is needed.
  - Reverting in link mode consists of restoring the pg_control file on the
    primaries, and rsyncing the source cluster mirrors to the primaries.
  - When the source cluster has no mirrors/standby:
    - reverting during initialize is allowed in both copy and link mode
    - reverting during execute is allowed in copy mode
    - reverting during execute is *not* allowed in link mode

```
  start <---- run migration
    |            scripts  |
run migration             |
  scripts                 ^
    |                     |
    V                     |
initialize ---> revert ----
    |                     ^
    V                     |
 execute  ----> revert ----
    |
    V
 finalize
    |
run migration 
  scripts
    |
    V
   done
```

Each substep within a step implements [crash-only idempotence](https://en.wikipedia.org/wiki/Crash-only_software).
This means that if an error occurs and is fixed then on rerun the step will 
succeed. This requires each substep to clean up any side effects it creates, 
or possibly check if the work has been done.

**Link vs. Copy Mode:**

ggupgrade inits a fresh target cluster "next to" the source cluster, and upgrades 
"into it" in-place using pg_upgrade's copy or link mode.

| Attribute | Copy Mode | Link Mode
| --- | --- | ---
| Description | Copy's source files to the target cluster. | Uses hard links to modify the source cluster data in place.
| Upgrade Time | Slow, since it copy's the data before upgrading. | Fast, since the data is modified in place.
| Disk Space | ~60% free disk space needed. | ~20% free disk space needed.
| Revert Speed | Fast, since the source cluster remains untouched. | Slow, since the source files have been modified the primaries and mirrors need to be rebuilt.
| Risk | Less risky since the source cluster is untouched. | More risky since the source cluster is modified.


## Getting Started

### Prerequisites

- Golang. See the top of [go.mod](go.mod) for the current version used.
- protoc. This is the compiler for the [grpc protobuf](https://grpc.io/)
  system which can be installed from the github repository.
  `https://github.com/protocolbuffers/protobuf/releases`.

### Setting up your IDE

#### Vim

Checkout [vim-go](https://github.com/fatih/vim-go) and [go-delve](https://github.com/go-delve/delve).

#### IntelliJ

##### Golang
- In Preferences > Plugins, install the "Go" plugin from JetBrains.

##### Imports
- Preferences > Editor > Code Style > Go > select "Imports" tab
  - uncheck "Use back quotes for imports"
  - uncheck "Add parentheses for a single import"
  - uncheck "Remove redundant import aliases"
  - Sorting type: gofmt
  - check "Move all imports in a single declaration"
  - check "Group stdlib imports"
    - check "Move all stdlib imports in a single group"
  - check "Group"
    - check "Current project packages"

##### Copyright
- Preferences > Editor > Copyright > Formatting
  - Select "Use custom formatting options"
  - For Comment Type: select "use line comment"
  - For Relative Location: select "Before other comments" and check "Add blank line after"

##### Formatting
- Preferences > Tools > Actions on Save
  - check "Reformat code" and "Optimize imports"

### Build and Test

```
make                        # build ggupgrade binary (same as make build)
make install-dependencies   # installs necessary developer dependencies and tools
make generate               # recompiles proto files to generate gRPC client and server code
make build                  # build ggupgrade binary
make install                # installs ggupgrade into $GOBIN
make lint                   # runs linter
make unit                   # runs unit test
```

When building ggupgrade for the first time, run the following series of commands:
```
make install-dependencies
make generate
make build
```

Unless you modify `.proto` files, it is enough to do his step only once. So on all subsequent builds you can run only `make build` (or just `make`).

After above setup, cross-compile with:
- `make build_linux`
- `make build_mac`

### Running

```
ggupgrade initialize --file ./ggupgrade_config
OR
ggupgrade initialize --source-gphome "$GPHOME" --target-gphome "$GPHOME" --source-master-port 6000 --mode link --disk-free-ratio 0 --seed-dir ~/workspace/ggupgrade/data-migration-scripts
ggupgrade execute
ggupgrade finalize
```

### Running Tests

#### Unit tests
```
make unit
```

#### Integration tests
Tests that run against the ggupgrade binary to verify the interaction between 
components. Before writing a new integration test please review the 
[README](https://github.com/GreengageDB/ggupgrade/blob/main/test/integration/README.md).
```
make integration
```

#### Acceptance tests
Tests more end-to-end acceptance-level behavior between components. Tests are
located in the `test` directory and use the isolation2 framework.
Please review the [integration/README](https://github.com/GreengageDB/ggupgrade/blob/main/test/integration/README.md).
```
# Requires a GPDB cluster installed and running
make acceptance
make pg-upgrade-tests
```

To run all tests in a suite:
```
go test -v ./test/acceptance/ggupgrade -run TestFinalize
```

To run a single test or set of tests:
```
go test -v ./test/acceptance/ggupgrade -run "ggupgrade finalize should"
```

#### All local tests
```
# Runs all local tests
make test --keep-going
```

#### End-to-End tests
Creates a Concourse pipeline that includes various multi-host X-to-Y upgrade and 
functional tests. These cannot be run locally.
```
make pipeline
```

#### Functional tests
Creates a Concourse pipeline for testing metadata and _any_ other SQL dump file.
See [ci/functional/README.md](https://github.com/GreengageDB/ggupgrade/blob/main/ci/functional/README.md) 
for specifics. These cannot be run locally.
```
make functional-pipeline
```

## Concourse Pipeline

To update the pipeline edit the yaml files in the `ci` directory and run 
`make pipeline`. 

The yaml files in the `ci` directory are concatenated to 
create `ci/generated/template.yml`. Next, `go generate ./ci` is executed which 
runs `go run ./parser/parse_template.go generated/template.yml generated/pipeline.yml`
to create `ci/generated/pipeline.yml`. None of the generated files `template.yml` 
or `pipeline.yml` are checked in.

To update the production pipeline locally checkout main and be sure to pull 
the latest code and fly with `PIPELINE_NAME=ggupgrade FLY_TARGET=prod make pipeline`

To make the pipeline publicly visible run `make expose-pipeline`. This will 
allow anyone to see the pipeline and its status. However, the task details will 
not be visible unless one logs into Concourse.

*Note:* If your dev pipeline is failing on the build job while verifying the rpm 
then the most likely cause is needing to sync the latest tags on origin with 
your remote. This allows the GPDB test rpm to have the correct version number. 
On your GPDB branch run the following:
```
$ git fetch --tags origin
$ git push --tags <yourRemoteName>
```
If you already flew a pipeline *before* pushing tags you will likely 
need to delete it, push tags, and re-fly as Concourse has some weird caching 
issues.


## Bash Completion

To enable tab completion of ggupgrade commands source the `cli/bash/ggupgrade.bash`
script from your `~/.bash_completion` config, or copy it into your system's 
completions directory such as  `/etc/bash_completion.d`.

## Log Locations
Logs are located on **_all hosts_**.

- ggupgrade logs: `$HOME/gpAdminLogs/ggupgrade`
  - After finalize the directory is archived with format `ggupgrade-<timestamp-upgradeID>`.
- pg_upgrade logs: `$HOME/gpAdminLogs/ggupgrade/pg_upgrade`
- greengage utility logs: `$HOME/gpAdminLogs`
- source cluster pg_log: `$MASTER_DATA_DIRECTORY/pg_log`
- target cluster pg_log: `$(ggupgrade config show --target-datadir)/pg_log`
  - The target cluster data directories are located next to the source cluster directories with the format `-<upgradeID>.<contentID>`

## Debugging
- Identify the High Level Failure
  - What mode was used - copy vs. link?
  - What step failed - initialize, execute, finalize, or revert?
  - What specific substep failed?
- Identify the Failing Host
  - Did the Hub (coordinator) vs. Agent (segment) fail?
  - What specific host failed?
- Identify the Failed Utility
  - Did ggupgrade fail, or an underlying utility such as pg_upgrade, gpinitsystem, gpstart, etc.?
- Identify the Specific Failure
  - Based on the error context and logs what is the specific error?

### Debugging Hub and Agent Processes
- Set a breakpoint in the CLI
  - For example in `cli/commands/initialize.go`, `execute.go`, or `finalize.go` right before the gRPC call to the hub.
- Run ggupgrade to hit the breakpoint in the CLI and start the hub process.
- When using intellij "Attach to Process" and select the hub and/or agent processes.
- Set additional breakpoints in the hub or agent code to aid in debugging.
- Continue execution on the CLI until the additional breakpoints in the hub or agent code are hit. Step through the code 
to debug.
- For faster iterations:
  - Make any local changes in the code
  - Rebuild with `make && make install`
  - Reload the new code with `ggupgrade restart-services` or manually stop and restart the hub.
  - Repeat the above breakpoints and attaching to the new processes as their PIDs have changed.
