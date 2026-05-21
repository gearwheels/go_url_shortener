package main

import "log/slog"

func main() {
	slog.Info("started")
	// os.Exit не вызывается — анализатор не должен ничего сообщать
}
