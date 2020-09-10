package main

import (
	"fmt"
	"kimo/client"
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
		{
			Name:  "client",
			Usage: "run client",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "host",
					Usage: "MySQL host",
				},
				cli.StringFlag{
					Name:  "user",
					Usage: "MySQL user",
				},
				cli.StringFlag{
					Name:  "password",
					Usage: "MySQL password",
				},
			},
			Action: func(c *cli.Context) error {
				host := c.String("host")
				user := c.String("user")
				password := c.String("password")
				err := client.Run(host, user, password)
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
