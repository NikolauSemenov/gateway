package main

import (
	"log/slog"
	"os"

	"gateway/internal/app"
)

// @title Gateway
// @version 1.0
// @description Сервис для проксирования http запросов.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With(
		slog.String("service", "gateway"),
	)

	serv, err := app.NewApp()
	if err != nil {
		log.Error("failed to initialize application", slog.Any("err", err))
		os.Exit(1)
	}

	if err := serv.Run(); err != nil {
		log.Error("http server stopped with error", slog.Any("err", err))
		os.Exit(1)
	}
}
