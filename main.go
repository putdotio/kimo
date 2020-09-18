package main

import (
	"kimo/config"
	"kimo/daemon"
	"kimo/server"
	"os"

	"github.com/cenkalti/log"
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
			log.Errorf("Cannot read config: %s\n", err)
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
			Name:  "daemon",
			Usage: "run daemon",
			Action: func(c *cli.Context) error {
				kimoDaemon := daemon.NewDaemon(&cfg.Daemon)
				err := kimoDaemon.Run()
				if err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name:  "server",
			Usage: "run server",
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
				kimoServer := server.NewServer(&cfg.Server)
				kimoServer.Config = &cfg.Server
				err := kimoServer.Run()
				if err != nil {
					return err
				}
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Errorln(err)
	}
}
