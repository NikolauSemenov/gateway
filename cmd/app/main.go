package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gateway/internal/app"
)

// @title Gateway
// @version 1.0
// @description Сервис для проксирования http запросов.
func main() {
	os.Exit(run())
}

func run() int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With(
		slog.String("service", "gateway"),
	)

	log.Info("initializing application")

	serv, err := app.NewApp()
	if err != nil {
		log.Error("failed to initialize application", slog.Any("err", err))
		return 1
	}

	// Канал для получения сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Канал для ошибок сервера
	errChan := make(chan error, 1)

	// Запуск сервера в отдельной горутине
	go func() {
		log.Info("starting http server")
		if err := serv.Run(); err != nil {
			errChan <- err
		}
	}()

	// Ожидание сигнала завершения или ошибки
	select {
	case sig := <-sigChan:
		log.Info("received shutdown signal", slog.String("signal", sig.String()))
	case err := <-errChan:
		log.Error("http server stopped with error", slog.Any("err", err))
		return 1
	}

	log.Info("shutting down server gracefully")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := serv.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown server gracefully", slog.Any("err", err))
		return 1
	}

	log.Info("server stopped successfully")
	return 0
}
