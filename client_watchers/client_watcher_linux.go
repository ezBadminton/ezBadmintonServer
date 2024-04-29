package client_watchers

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// WatchClientForExit on linux makes a process control (prctl) syscall to receive a signal upon
// death of its parent process which should be the ezBadminton client.
func WatchClientForExit() {
	_, _, err := syscall.Syscall(syscall.SYS_PRCTL, uintptr(syscall.PR_SET_PDEATHSIG), uintptr(syscall.SIGHUP), 0)
	if err != 0 {
		log.Printf("PRCTL syscall failed. Error: %v", err)
		return
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGHUP)
	go func() {
		<-signalChannel
		os.Exit(0)
	}()
}
