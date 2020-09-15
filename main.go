package main

import (
	"fmt"
	"kimo/client"
	"kimo/config"
	"kimo/server"
	"os"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	var cfg = config.NewConfig()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config, c",
			Value:  "/etc/kimo.toml",
			Usage:  "configuration file path",
			EnvVar: "KIMO_CONFIG",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "enable debug log",
		},
		cli.BoolFlag{
			Name:  "no-debug, D",
			Usage: "disable debug log",
		},
	}
	app.Before = func(c *cli.Context) error {
		err := cfg.ReadFile(c.GlobalString("config"))
		if err != nil {
			// TODO: make this debug log
			fmt.Println("Cannot read config:", err)
		}
		if c.IsSet("debug") {
			cfg.Debug = true
		}
		if c.IsSet("no-debug") {
			cfg.Debug = false
		}
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "server",
			Usage: "run server",
			Action: func(c *cli.Context) error {
				kimoServer := server.NewServer(&cfg.Server)
				err := kimoServer.Run()
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
				kimoClient := client.NewClient(&cfg.Client)
				kimoClient.Config = &cfg.Client
				err := kimoClient.Run()
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
