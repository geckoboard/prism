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
			ArgsUsage:   "path_to_main_file",

			Action: cmd.ProfileProject,
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "target, t",
					Value: &cli.StringSlice{},
					Usage: "fully qualified function name to profile",
				},
				cli.StringFlag{
					Name:  "build-cmd",
					Value: "go build -o profile-target",
					Usage: "project build command",
				},
				cli.StringFlag{
					Name:  "run-cmd",
					Value: "profile-target",
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
		},
		{
			Name:        "diff",
			Usage:       "visually compare profiles",
			Description: ``,
			ArgsUsage:   "profile1 profile2 [...profile_n]",
			Action:      cmd.DiffProfiles,
		},
	}

	app.Run(os.Args)
}
