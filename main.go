package main

import (
	"fmt"
	"kimo/server"
	"os"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:  "server",
			Usage: "run server",
			Action: func(c *cli.Context) error {
				err := server.Run()
				if err != nil {
					return err
				}
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
