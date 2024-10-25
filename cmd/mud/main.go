package main

import (
	"context"

	"github.com/pixil98/go-log/log"
	"github.com/pixil98/go-mud/cmd/mud/command"
	"github.com/pixil98/go-service/service"
)

func main() {
	logger := log.NewLogger()

	app, err := service.NewApp(&command.Config{}, command.BuildWorkers)
	if err != nil {
		logger.WithError(err).Fatal("creating application")
	}

	err = app.Run(context.Background())
	if err != nil {
		logger.WithError(err).Fatal("running application")
	}

	logger.Info("exiting")
}
