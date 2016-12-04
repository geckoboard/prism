![prism logo](https://drive.google.com/uc?export=download&id=0B0tIAvKWmwa5S3J6eUdvUkVyc1U)

[![CircleCI](https://circleci.com/gh/geckoboard/prism.svg?style=svg)](https://circleci.com/gh/geckoboard/prism)
[![Coverage Status](https://coveralls.io/repos/github/geckoboard/prism/badge.svg?branch=master)](https://coveralls.io/github/geckoboard/prism?branch=master)
[![GoDoc](https://godoc.org/github.com/geckoboard/prism?status.svg)](https://godoc.org/github.com/geckoboard/prism)

Prism is a tool that allows you to collect and analyze profiling data from your
go applications. It is not meant to be  used as a replacement for tools like [pprof](https://golang.org/pkg/net/http/pprof/)
but rather as an alternative for cases when you:
- want to collect profile data per **go-routine**,
- do not want to manually modify your project's source to include the profiler instrumentation code,
- want to quickly compare the performance of code between a set of git commits

## Installing

Prism requires go 1.5+. To install it run:

`go get github.com/geckoboard/prism/... && go install github.com/geckoboard/prism`.

You can then run `prism -h` for details on the command line arguments.

## How it works

Prism receives as input the path to your project and a list of **fully qualified** (FQ)
function targets to be profiled. It then creates a temporary go workspace containing 
a copy of your project and analyzes its sources looking for the specified profile targets. 
	
### Target call graph construction

For each found target, prism uses [Rapid Type Analysis](https://godoc.org/golang.org/x/tools/go/call graph/rta) (RTA)
to discover its call graph. The call graph includes all functions potentially reachable 
either directly or indirectly via the profile target. Whenever RTA encounters an interface 
it will properly expand the set of reachable functions to include the types that 
satisfy that particular interface. 

In the following example, running RTA on `DoStuff` will treat the implementation
of `Work()` for both `aProvider` and `otherProvider` as reachable and include both
in the generated call graph.

```golang
type Provider interface {
	Work()
}

func provider(type string) Provider {
	switch type {
		case "A": 
			return &aProvider{}
		default:
			return &otherProvider{}
	}
}

// This is our profile target
func DoStuff(providerType string) {
	provider(providerType).Work()
}
```

The call graph is then *pruned* to remove any functions that do not belong to the 
project package or any of its sub-packages. The prune step is required as prism 
is not able to hook code imported from external packages; prism can only parse 
and modify code present in the cloned project folder (and optionally in vendored
packages).

### Profiler injection

Once the call graphs for each profile target have been generated, prism will 
parse each source file into an abstract syntax tree (AST) and visit each node 
looking for function declarations matching the call graph entries. Each matched 
function is modified to inject the following profiler hooks:
- BeginProfile/Enter
- EndProfile/Leave (using a [defer](https://golang.org/doc/effective_go.html#defer) statement)

The Begin/End profile hooks are used for the profile targets whereas the Enter/Leave 
hooks are used for any function reachable via the profile target's call graph.

In addition, prism will also hook the `main()` function of the project and 
inject some additional hooks to init/configure the profiler (see [profile](#profile) command below)
and ensure that all captured profiles are properly processed before the program 
exits.

### Building/running the patched project 

Once the profiler code has been injected into the project copy, prism will build
and run it capturing any generated profiling data. Both the `build` and the `run`
steps use a modified `GOPATH` with the temporary go workspace prepended. This 
allows the compiler to use the patched version of the project and its 
sub-packages while still being able to lookup external packages residing 
in the original `GOPATH`.

The collected data can be displayed using the [print](#print) command or 
compared with previously collected data using the [diff](#diff) command.

### Caveats

#### Profiling inlineable functions

Our profiler relies on the injection of hooks to each tracked function for 
collecting data. If your code uses inlineable functions, the profiler will 
**overestimate** the total time spent in the function as injection of the 
hook invocations will prevent the compiler from inlining those functions.

#### Profiler overhead

The profiler hooks introduce overhead to all profiled targets. This overhead 
adds up if your code invokes a profiled function a large number of times and 
is likely to slow down the execution of the program.

The implementation of the profiler goes into [great lengths](https://github.com/geckoboard/prism/pull/9) 
to ensure that this overhead is tracked and accounted for when calculating the 
various time-related metrics. Still, our approach to tracking overhead is not 
100% accurate and may introduce a small error; a few μsec up to a few ms
depending on the number of invocations of each profiled function. If you need 
more accuracy we recommend using [pprof](https://golang.org/pkg/net/http/pprof/)
instead.

## Using prism

### profile

The `profile` command allows you to clone your project, inject profile hooks to a 
set of target functions, build and run the project to collect profiling data.

```
Usage:
prism profile [command options] path_to_project

Example:
prism profile -t github.com/example/test/main $GOPATH/example/test
```

#### Constructing the fully qualified (FQ) target names

When constructing the FQ target name the following rules apply:

If the function **does not use a receiver**, e.g. `func foo(){...}` concatenate:
- the name of your project's package, e.g. `github.com/prism`
- a '/' character
- the name of the function, e.g. `foo`, yielding the FQ target: `github.com/prism/foo`

If the function **uses** a receiver (pointer or non-pointer), e.g `func (a *A) foo(){...}` concatenate:
- the name of your project's package, e.g. `github.com/prism`
- a '/' character
- the name of the receiver type, e.g. `A`
- a '.' character
- the name of the function, e.g. `foo`, yielding the FQ target: `github.com/prism/A.foo`

#### Supported options

The following options can be used with the `profile` command (see `prism profile -h` for more details):

| Option                           | Default                  | Description           
|----------------------------------|--------------------------|-------------------
| --build-cmd value                |                          | an optional build command to execute before running the patched project
| --run-cmd value                  | `find . -d 1 -type f -name *\\.go ! -name *_test\\.go -exec go run {} +` | a command for running the patched project; e.g. `make run`
| --profile-target value, -t value |                          | a FQ target name to be hooked; this option may be specified multiple times
| --profile-dir value              | $HOME/prism              | the folder where captured profiles will be stored
| --profile-label value            |                          | a label used for tagging captured profiles; e.g. your commit SHA
| --profile-vendored-pkg regex     |                          | also hook functions in vendored packages matching this regex; this option may be specified multiple times
| --output-dir value -o value      | System's temp folder     | the directory for storing the copied project files
| --preserve-output                |                          | keep the cloned project copy instead of deleting it (default) after prism exits
| --no-ansi                        |                          | disable color output; prism does this automatically if it detects a non-TTY terminal

#### Running the profiled project 

Once prism has injected the profile hooks, it wil run the patched program saving 
captured profiles into the folder specified by the `--profile-dir` option. While 
the project is running, prism will relay the following signals to the running process:
- HUP
- INT
- TERM
- QUIT

This allows you to stop long-running processes (e.g. if the profiled project 
implements an http server) and return control back to prism by pressing `CTRL+C`.

#### Profile output

All captured profiles are stored as JSON files in the directory specified by the 
`--profile-dir` command. The generated profile filenames match the pattern 
`profile-target-timestamp-goid.json` where:
- `target` is the fully qualified target name (with slashes replaced by underscores)
- `timestamp` is the UTC timestamp (in nanoseconds) when the profile was captured
- `goid` is the ID of the go-routine which invoked the profile target.

This format makes it very easy to use shell expansion and get a time-sorted
list of profiles to feed into the `diff` command.

### print

The `print` command allows you to display a captured profile into tabular form.

```
Usage:
prism print [command options] profile

Example:
prism print profile-before.json

+-----------------------------------------------------+-----------+-----------+-----------+-----------+---------+
| Before change - call stack                          |     total |       min |      mean |       max |   invoc |
+-----------------------------------------------------+-----------+-----------+-----------+-----------+---------+
| + github.com/geckoboard/test/main                   | 284.00 ms | 284.00 ms | 284.00 ms | 284.00 ms |       1 |
| | + github.com/geckoboard/test/processor.processRow | 158.30 ms |   1.10 ms |   1.53 ms |   1.98 ms | 1000000 |
| | | - github.com/geckoboard/test/processor.encrypt  | 150.00 ms |   1.00 ms |   1.40 ms |   1.80 ms | 1000000 |
+-----------------------------------------------------+-----------+-----------+-----------+-----------+---------+

prism print --display-format=percent profile-before.json 

+-----------------------------------------------------+--------+--------+--------+--------+---------+
| Before change - call stack                          |  total |    min |   mean |    max |   invoc |
+-----------------------------------------------------+--------+--------+--------+--------+---------+
| + github.com/geckoboard/test/main                   | 100.0% | 100.0% | 100.0% | 100.0% |       1 |
| | + github.com/geckoboard/test/processor.processRow |  55.7% |   0.4% |   0.5% |   0.7% | 1000000 |
| | | - github.com/geckoboard/test/processor.encrypt  |  52.8% |   0.4% |   0.5% |   0.6% | 1000000 |
+-----------------------------------------------------+--------+--------+--------+--------+---------+
```

#### Supported options

The following options can be used with the `print` command (see `prism print -h` for more details):

| Option                           | Default                  | Description           
|----------------------------------|--------------------------|-------------------
| --display-columns, --dc value    | total,min,mean,max,invocations | the columns to include in the output; see [supported column types](#supported-column-types) for the list of supported values
| --display-format, --df value     | time                     | set format for columns containing time values; supported options are: `time` and `percent`
| --display-unit, --du value       | ms                       | set time unit format for columns containing time values; supported options are: `auto`, `ms`, `us`, `ns`
| --display-threshold value        | 0                        | mask time-related entries less than `value`; uses the same unit as `--display-unit` unless `--display-format` is `percent` where `value` is used to threshold displayed percentages
| --no-ansi                        |                          | disable color output; prism does this automatically if it detects a non-TTY terminal

#### Supported column names

The following column types are supported by the `--display-columns` option when 
running either the `print` or `diff` prism commands:

| Column type | Description
|-------------|----------------
| invocations | number of invocations
| total       | total time spent in function for all its invocations 
| min         | min invocation time
| max         | max invocation time
| mean        | mean invocation time
| median      | median invocation time
| p50         | 50th percentile of invocation total time 
| p75         | 75th percentile of invocation total time 
| p90         | 90th percentile of invocation total time 
| p99         | 99th percentile of invocation total time 
| stddev      | standard deviation for invocation time

### diff

The `diff` command allows you compare a set of profiles and display the results 
in tabular form. It expects *two* or more profiles as its input. The first profile 
argument will be treated as the baseline and each other profile argument will be 
compared against the baseline.

When comparing values, prism will use symbols `↑`, `↓` or `≈` to indicate whether a 
profile value is `greater`, `less` or `approximately equal` to the baseline profile
and also format the difference as a percent.

```
Usage:
prism diff [command options] baseline_profile profile_1 ... profile_n

Example:
prism diff profile-before.json profile-after.json

+-----------------------------------------------------+---------------------------------------------------------+-------------------------------------------------------------------------------------------------+
|                                                     | Before change - baseline                                | After change                                                                                    |
+-----------------------------------------------------+---------------------------------------------------------+-------------------------------------------------------------------------------------------------+
| call stack                                          |     total |       min |      mean |       max |   invoc |               total |                 min |                mean |                 max |   invoc |
+-----------------------------------------------------+-----------+-----------+-----------+-----------+---------+---------------------+---------------------+---------------------+---------------------+---------+
| - github.com/geckoboard/test/main                   | 284.00 ms | 284.00 ms | 284.00 ms | 284.00 ms |       1 | 254.00 ms (↓ 11.8%) | 254.00 ms (↓ 11.8%) | 254.00 ms (↓ 11.8%) | 254.00 ms (↓ 11.8%) |       1 |
| | - github.com/geckoboard/test/processor.processRow | 158.30 ms |   1.10 ms |   1.53 ms |   1.98 ms | 1000000 | 128.30 ms (↓ 23.4%) |   1.11 ms  (↑ 0.9%) |   1.15 ms (↓ 33.0%) |   1.20 ms (↓ 65.0%) | 1000000 |
| | | + github.com/geckoboard/test/processor.encrypt  | 150.00 ms |   1.00 ms |   1.40 ms |   1.80 ms | 1000000 | 120.00 ms (↓ 25.0%) |   1.00 ms       (≈) |   1.10 ms (↓ 27.3%) |   1.82 ms  (↑ 1.1%) | 1000000 |
+-----------------------------------------------------+-----------+-----------+-----------+-----------+---------+---------------------+---------------------+---------------------+---------------------+---------+
```

#### Supported options

The following options can be used with the `diff` command (see `prism diff -h` for more details):

| Option                           | Default                  | Description           
|----------------------------------|--------------------------|-------------------
| --display-columns, --dc value    | total,min,mean,max,invocations | the columns to include in the output; see [supported column types](#supported-column-types) for the list of supported values
| --display-unit, --du value       | ms                       | set time unit format for columns containing time values; supported options are: `auto`, `ms`, `us`, `ns`
| --display-threshold value        | 0                        | mask comparison entries with abs delta time less than `value`; uses the same unit as `--display-unit`
| --no-ansi                        |                          | disable color output; prism does this automatically if it detects a non-TTY terminal

## Running prism for a range of Git commits

One particular use of prism is to collect and diff profiling data for a sequence
of Git commits. To this end, we have come up with a simple shell script to automate
that. 

The script checks out all commits between the two hashes and runs prism profile for 
each commit using its SHA as the profile label, storing the captured profiles 
in a temporary directory. Once all profiles have been collected it will runs 
a prism diff command to print a table comparing each profile against the one 
obtained for the first commit.

```bash
#!/bin/bash

set -euo pipefail

if [ "$#" -lt 3 ]; then
    echo "Generate prism profiles for a sequence of commit SHAs and output their diff"
    echo
    echo "Usage: "
    echo "git-prism.sh start_commit end_commit target..."
    echo
    exit 1
fi

if [[ ! -z $(git status -s) ]]; then
    echo "git-prism: target repo is dirty; please commit or stash your changes and try again"
    exit 1
fi

CUR_BRANCH=`git rev-parse --abbrev-ref HEAD`
START_COMMIT=$1
END_COMMIT=$2
shift 2

TARGETS=""
for target in "$@"
do
    TARGETS=${TARGETS}" -t $target"
done

# Setup tmp dir for profiles
TMPDIR=`mktemp -d`
trap 'git checkout "$CUR_BRANCH" ; rm -rf "$TMPDIR" ; exit 255;' EXIT

# Generate profile for each SHA
for sha in `git rev-list --reverse --abbrev-commit $START_COMMIT $END_COMMIT`; do
    git checkout -q $sha && prism profile --profile-dir="$TMPDIR" --profile-label="$sha" $TARGETS ./
done

# Output diff
prism diff $TMPDIR/*.json
```

You can run this script directly or define a Git alias such as:

`git config --global alias.prismdiff '!bash /usr/local/bin/prism-diff.sh'`

which allows you to get a diff for all commits between two SHAs by running: 

`git prismdiff start_SHA end_SHA target_1 ... targetN`

## Related articles

A brief introduction on prism and a simple example of its use can be found in 
the [introducing prism blog post](https://medium.com/geckoboard-under-the-hood/introducing-prism-9c08e9926755).


## Contributing 

Please read our [contributing guide](CONTRIBUTING.md).

## License

Prism is released under the [MIT license](LICENSE).
