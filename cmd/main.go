package main

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"share/internal/service"
)

func main() {
	services := service.NewServices()

	app := &cli.App{
		Name:  "share",
		Usage: "easily share files with s3",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "copy",
				Aliases: []string{"c"},
				Usage:   "copy public url to clipboard",
			},
		},
		Action: func(c *cli.Context) error {
			if len(c.Args().First()) == 0 {
				cli.ShowAppHelpAndExit(c, 0)
			}
			return services.Share().Share(c.Args().First(), c.Bool("copy"))
		},
		Commands: []*cli.Command{
			{
				Name:  "setup",
				Usage: "configure bucket name",
				Action: func(cCtx *cli.Context) error {
					return services.Config().Setup(true, true)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
