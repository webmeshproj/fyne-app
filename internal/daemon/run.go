/*
Copyright 2023 Avi Zimmerman <avi.zimmerman@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package daemon

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

// Run runs the helper daemon.
func Run() {
	server := NewServer()
	log := slog.Default()
	go func() {
		log.Info("listening for daemon requests", "path", getSocketPath())
		if err := server.ListenAndServe(); err != nil {
			log.Error("daemon server error", "error", err.Error())
			os.Exit(1)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		log.Error("daemon shutdown error", "error", err.Error())
		os.Exit(1)
	}
}
