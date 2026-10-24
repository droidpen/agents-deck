package main

import (
	"fmt"
	"time"
)

func stopDaemon() error {
	if err := call("POST", "/local/stop", nil, nil); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := call("GET", "/v1/state", nil, nil); err != nil {
			fmt.Println("daemon stopped; agents keep running in tmux")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not stop within 10 seconds")
}
