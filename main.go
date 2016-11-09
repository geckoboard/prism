package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/geckoboard/prism/cmd"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "prism"
	app.Usage = "profiler injector and analysis tool"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name:        "profile",
			Usage:       "clone go project and inject profiler",
			Description: `Create a temp copy of a go project, attach profiler, build and run it.`,
			ArgsUsage:   "path_to_project",

			Action: cmd.ProfileProject,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "build-cmd",
					Value: "",
					Usage: "project build command",
				},
				cli.StringFlag{
					Name:  "run-cmd",
					Value: `find . -d 1 -type f -name *\.go ! -name *_test\.go -exec go run {} +`,
					Usage: "project run command",
				},
				cli.StringFlag{
					Name:  "output-dir, o",
					Value: os.TempDir(),
					Usage: "path for storing patched project version",
				},
				cli.BoolFlag{
					Name:  "preserve-output",
					Usage: "preserve patched project post build",
				},
				cli.StringSliceFlag{
					Name:  "profile-target, t",
					Value: &cli.StringSlice{},
					Usage: "fully qualified function name to profile",
				},
				cli.StringFlag{
					Name:  "profile-dir",
					Usage: "specify the output dir for captured profiles",
					Value: defaultOutputDir(),
				},
				cli.StringFlag{
					Name:  "profile-label",
					Usage: `specify a label to be attached to captured profiles and displayed when using the "print" or "diff" commands`,
				},
				cli.StringSliceFlag{
					Name:  "profile-vendored-pkg",
					Usage: "inject profile hooks to any vendored packages matching this regex. If left unspecified, no vendored packages will be hooked",
					Value: &cli.StringSlice{},
				},
				cli.BoolFlag{
					Name:  "no-ansi",
					Usage: "disable ansi output",
				},
			},
		},
		{
			Name:        "print",
			Usage:       "pretty-print profile",
			Description: ``,
			ArgsUsage:   "profile",
			Action:      cmd.PrintProfile,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "columns",
					Value: "total,min,mean,max,invocations",
					Usage: fmt.Sprintf("columns to include in the output; supported options: %s", cmd.SupportedColumnNames()),
				},
				cli.Float64Flag{
					Name:  "threshold",
					Value: 0.0,
					Usage: "only show measurements for entries whose time exceeds the threshold",
				},
				cli.BoolFlag{
					Name:  "no-ansi",
					Usage: "disable ansi output",
				},
			},
		},
		{
			Name:        "diff",
			Usage:       "visually compare profiles",
			Description: ``,
			ArgsUsage:   "profile1 profile2 [...profile_n]",
			Action:      cmd.DiffProfiles,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "columns",
					Value: "total,min,mean,max,invocations",
					Usage: fmt.Sprintf("columns to include in the diff output; supported options: %s", cmd.SupportedColumnNames()),
				},
				cli.Float64Flag{
					Name:  "threshold",
					Value: 0.0,
					Usage: "only compare entries whose time difference exceeds the threshold",
				},
				cli.BoolFlag{
					Name:  "no-ansi",
					Usage: "disable ansi output",
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
}

// Get default output dir for profiles.
func defaultOutputDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return usr.HomeDir + "/prism"
}
