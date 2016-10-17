package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/geckoboard/prism/cmd"
	"github.com/urfave/cli"
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
				cli.StringSliceFlag{
					Name:  "target, t",
					Value: &cli.StringSlice{},
					Usage: "fully qualified function name to profile",
				},
				cli.StringFlag{
					Name:  "build-cmd",
					Value: "",
					Usage: "project build command",
				},
				cli.StringFlag{
					Name:  "run-cmd",
					Value: `go run main.go`,
					Usage: "project run command",
				},
				cli.StringFlag{
					Name:  "output-folder, o",
					Value: os.TempDir(),
					Usage: "path for storing patched project version",
				},
				cli.BoolFlag{
					Name:  "preserve-output",
					Usage: "preserve patched project post build",
				},
				cli.StringFlag{
					Name:  "profile-folder",
					Usage: "specify the output folder for captured profiles",
					Value: defaultOutputDir(),
				},
				cli.StringSliceFlag{
					Name:  "profile-vendored-pkg",
					Usage: "inject profile hooks to any vendored packages matching this regex. If left unspecified, no vendored packages will be hooked",
					Value: &cli.StringSlice{},
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
					Value: "total,avg,min,max,invocations",
					Usage: "columns to include in the printout",
				},
				cli.Float64Flag{
					Name:  "threshold",
					Value: 0.0,
					Usage: "only show measurements for entries whose time exceeds the threshold",
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
					Value: "total,avg,min,max,invocations",
					Usage: "columns to include in diff table",
				},
				cli.Float64Flag{
					Name:  "threshold",
					Value: 0.0,
					Usage: "only compare entries whose time difference exceeds the threshold",
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
