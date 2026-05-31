package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/isox/internal/config"
	"github.com/isox/internal/pipeline"
)

func main() {
	cfgPath := flag.String("config", "config/isox.conf", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	log.Printf("starting isox — workers: %d, MIP: %s", cfg.Global.Workers, cfg.Downstream.Addr)

	p, err := pipeline.New(cfg)
	if err != nil {
		log.Fatalf("error creating pipeline: %v", err)
	}

	p.Start()
	log.Printf("router started (pid %d)", os.Getpid())

	quit := make(chan os.Signal, 1)
	reload := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(reload, syscall.SIGHUP)

	for {
		select {
		case <-quit:
			log.Println("shutting down...")
			p.Stop()
			log.Println("stopped")
			return

		case <-reload:
			log.Printf("received SIGHUP — reloading config from %s", *cfgPath)
			newCfg, err := config.Load(*cfgPath)
			if err != nil {
				log.Printf("reload failed: %v — keeping current config", err)
				continue
			}
			p.Reload(newCfg)
		}
	}
}
