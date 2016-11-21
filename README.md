# prism
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

`go install github.com/geckoboard/prism`.

You can then run `prism -h` for details on the command line arguments.

## How it works

Prism receives as input the path to your project and a list of **fully qualified** (FQ)
function targets to be profiled. It then creates a temporary go workspace containing 
a copy of your project and analyzes its sources looking for the specified profile targets. 
	
### Target callgraph construction

For each found target, prism uses [Rapid Type Analysis](https://godoc.org/golang.org/x/tools/go/callgraph/rta) (RTA)
to discover its callgraph. The callgraph includes all functions potentially reachable 
either directly or indirectly via the profile target. Whenever RTA encounters an interface 
it will properly expand the set of reachable functions to include the types that 
satisfy that particular interface. 

In the following example, running RTA on `DoStuff` will treat the implementation
of `Work()` for both `aProvider` and `otherProvider` as reachable and include both
in the generated callgraph.

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

The callgraph is then *pruned* to remove any functions that do not belong to the 
project package or any of its sub-packages. The prune step is required as prism 
is not able to hook code imported from external packages; prism can only parse 
and modify code present in the cloned project folder (and optionally in vendored
packages).

### Profiler injection

Once the callgraphs for each profile target have been generated, prism will 
parse each source file into an abstract syntax tree (AST) and visit each node 
looking for function declarations matching the callgraph entries. Each matched 
function is modified to inject the following profiler hooks:
- BeginProfile/Enter
- EndProfile/Leave (using a [defer](https://golang.org/doc/effective_go.html#defer) statement)

The Begin/End profile hooks are used for the profile targets whereas the Enter/Leave 
hooks are used for any function reachable via the profile target's callgraph.

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

To attempt to quantify this error we run a series of benchmarks on an idle 
machine where we varied the number of invocations and compared the profiler
timings with the time required to run the same code without the profiler hooks 
in place. These are the results:

| Fn invocation count | Total skew  | Skew per invocation 
|---------------------|-------------|--------------------
| 1000                | 3 μsec      | 3 ns
| 100000              | 2 msec      | 20 ns 
| 1000000             | 28 msec     | 28 ns

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
| --buld-cmd value                 |                          | an optional build command to execute before running the patched project
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
prism print $HOME/prism/profile.json

+----------------------------+-----------+-----------+-----------+-----------+-------+
| Before change - call stack |     total |       min |      mean |       max | invoc |
+----------------------------+-----------+-----------+-----------+-----------+-------+
| + main                     | 120.00 ms | 120.00 ms | 120.00 ms | 120.00 ms |     1 |
| | - foo                    | 120.00 ms |  10.00 ms |  60.00 ms | 110.00 ms |     2 |
+----------------------------+-----------+-----------+-----------+-----------+-------+

prism print --display-format=percent $HOME/prism/profile.json 

+-------------------------+--------+--------+--------+--------+-------+
| With Label - call stack |  total |    min |   mean |    max | invoc |
+-------------------------+--------+--------+--------+--------+-------+
| + main                  | 100.0% | 100.0% | 100.0% | 100.0% |     1 |
| | - foo                 | 100.0% |   8.3% |  50.0% |  91.7% |     2 |
+-------------------------+--------+--------+--------+--------+-------+
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

When comparing values, prism will use symbols `>`, `<` or `=` to indicate whether a 
profile value is `greater`, `less` or `equal` to the baseline profile. A speed
factor is also included in the output to indicate how  `faster` or `slower` the 
entry is compared to the baseline: values less than `1x` indicate that the new 
entry runs slower, equal to `1x` indicate the entry runs the same and values 
greater than `1x` indicate the new entry runs faster.

```
Usage:
prism diff [command options] baseline_profile profile_1 ... profile_n

Example:
prism diff $HOME/prism/original.json $HOME/prism/after-change.json

+------------+--------------------------------------------+-------------------------------------------------------------------------------------+
|            | Original - baseline                        | After changes                                                                       |
+------------+--------------------------------------------+-------------------------------------------------------------------------------------+
| call stack |   total |    min |   mean |    max | invoc |              total |                min |            mean |             max | invoc |
+------------+---------+--------+--------+--------+-------+--------------------+--------------------+-----------------+-----------------+-------+
| - main     | 120.00  | 120.00 | 120.00 | 120.00 |     1 | 10.00 ms (< 12.0x) | 10.00 ms (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) |     1 |
| | + foo    | 120.00  |  10.00 |  60.00 | 110.00 |     2 | 10.00 ms (< 12.0x) |  4.00 ms  (< 2.5x) |  5.00 (< 12.0x) |  6.00 (< 18.3x) |     2 |
+------------+---------+--------+--------+--------+-------+--------------------+--------------------+-----------------+-----------------+-------+
```


#### Supported options

The following options can be used with the `diff` command (see `prism diff -h` for more details):

| Option                           | Default                  | Description           
|----------------------------------|--------------------------|-------------------
| --display-columns, --dc value    | total,min,mean,max,invocations | the columns to include in the output; see [supported column types](#supported-column-types) for the list of supported values
| --display-format, --df value     | time                     | set format for columns containing time values; supported options are: `time` and `percent`
| --display-unit, --du value       | ms                       | set time unit format for columns containing time values; supported options are: `auto`, `ms`, `us`, `ns`
| --display-threshold value        | 0                        | mask comparison entries with abs delta time less than `value`; uses the same unit as `--display-unit` unless `--display-format` is `percent` where `value` is used to threshold the abs delta difference percent
| --no-ansi                        |                          | disable color output; prism does this automatically if it detects a non-TTY terminal

## License

Prism is released under the [MIT license](LICENSE).
