package main

import (
	"kimo/agent"
	"kimo/config"
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
		cli.BoolFlag{
			Name:  "no-debug, D",
			Usage: "disable debug log",
		},
	}
	app.Before = func(c *cli.Context) error {
		err := cfg.ReadFile(c.GlobalString("config"))
		if err != nil {
			log.Errorf("Cannot read config: %s\n", err)
		}
		if c.IsSet("debug") {
			cfg.Debug = true
		}
		if c.IsSet("no-debug") {
			cfg.Debug = false
		}

		if cfg.Debug {
			log.SetLevel(log.DEBUG)
		} else {
			log.SetLevel(log.INFO)
		}

		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "agent",
			Usage: "run agent",
			Action: func(c *cli.Context) error {
				kimoAgent := agent.NewAgent(cfg)
				err := kimoAgent.Run()
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
