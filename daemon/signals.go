package main

import (
	"os"
	"os/signal"
	"syscall"
)

func registerSignals(c chan<- os.Signal) {
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
}
