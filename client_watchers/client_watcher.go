//go:build !(windows || linux)

package client_watchers

func WatchClientForExit() {}
