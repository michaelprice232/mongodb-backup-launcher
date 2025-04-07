package main

import (
	"log/slog"
	"os"

	"github.com/michaelprice232/mongodb-backup-launcher/config"
	"github.com/michaelprice232/mongodb-backup-launcher/internal/service"
)

func main() {
	conf, err := config.NewConfig()
	if err != nil {
		slog.Error("creating config", "error", err.Error())
		os.Exit(1)
	}

	s, err := service.NewService(conf)
	if err != nil {
		slog.Error("creating service", "error", err.Error())
		os.Exit(2)
	}

	err = s.Run()
	if err != nil {
		slog.Error("running the service", "error", err.Error())
		os.Exit(3)
	}
}
