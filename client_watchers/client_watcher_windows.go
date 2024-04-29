package client_watchers

import (
	"log"
	"os"

	"github.com/Microsoft/go-winio"
)

// WatchClientForExit on Windows makes a read call to a named pipe (https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes)
// that the ezBadminton client opens on start. Nothing is ever written to it so the read call blocks.
// Exit of the client is detected when the read call completes because the named pipe is closed.
func WatchClientForExit() {
	heartbeat, err := winio.DialPipe("\\\\.\\pipe\\ezbadmintonheartbeat", nil)
	if err != nil {
		log.Printf("Could not open named pipe. Error: %v", err.Error())
		return
	}

	go func() {
		heartbeat.Read(make([]byte, 1)) // Blocking read call
		os.Exit(0)
	}()
}
