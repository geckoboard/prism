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
			Usage:       "inject profiler to go project",
			Description: ``,
			ArgsUsage:   "path_to_main_file",
			Action:      cmd.ProfileProject,
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "target, t",
					Value: &cli.StringSlice{},
					Usage: "fully qualified function name to profile",
				},
				/*
					cli.StringFlag{
						Name:  "build-cmd",
						Value: "go build",
						Usage: "project build command",
					},
					cli.StringFlag{
						Name:  "post-build-cmd",
						Value: "",
						Usage: "post build command",
					},
				*/
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
	}

	app.Run(os.Args)
}
