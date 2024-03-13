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
		Action: func(c *cli.Context) error {
			return services.Share().Share(c.Args().First())
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
