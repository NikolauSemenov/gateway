package main

import (
	"gateway/internal/app"
	"log"
)

// @title Gateway
// @version 1.0
// @description Сервис для проксирования http запросов.
func main() {
	serv, err := app.NewApp()
	if err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}
	if err := serv.Run(); err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}

}
