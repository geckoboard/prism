package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/geckoboard/prism/cmd"
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
				cli.IntFlag{
					Name:  "threshold",
					Value: 0,
					Usage: "only compare entries whose time difference exceeds the threshold",
				},
			},
		},
	}

	app.Run(os.Args)
}
