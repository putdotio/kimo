package main

import (
	"kimo/config"
	"kimo/daemon"
	"kimo/server"
	"os"

	"github.com/cenkalti/log"
	"github.com/urfave/cli"
)

func init() {
	log.DefaultHandler.SetLevel(log.DEBUG)
}

func main() {
	app := cli.NewApp()
	var cfg = config.NewConfig()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "/etc/kimo.toml",
			Usage: "configuration file path",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "enable debug log",
		},
	}
	app.Before = func(c *cli.Context) error {
		err := cfg.ReadFile(c.GlobalString("config"))
		if err != nil {
			log.Errorf("Cannot read config: %s\n", err)
		}
		if c.IsSet("debug") {
			cfg.Debug = true
		} else {
			cfg.Debug = false
		}
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "daemon",
			Usage: "run daemon",
			Action: func(c *cli.Context) error {
				kimoDaemon := daemon.NewDaemon(cfg)
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
			Action: func(c *cli.Context) error {
				kimoServer := server.NewServer(cfg)
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
