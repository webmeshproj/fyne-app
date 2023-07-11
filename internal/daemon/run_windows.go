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
	"fmt"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

const svcName = "webmesh-helper"

func Run(insecure bool) {
	elog, err := eventlog.Open(svcName)
	if err != nil {
		return
	}
	defer elog.Close()
	elog.Info(1, fmt.Sprintf("starting %s service", svcName))
	err = svc.Run(svcName, &webmeshHelper{log: elog, insecure: insecure})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", svcName, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", svcName))
}

type webmeshHelper struct {
	log      debug.Log
	insecure bool
}

func (m *webmeshHelper) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	errs := make(chan error)
	daemon := NewServer(m.insecure)
	go func() {
		defer close(errs)
		if err := daemon.ListenAndServe(); err != nil {
			errs <- err
		}
	}()
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case err := <-errs:
			if err != nil {
				m.log.Error(1, err.Error())
			}
			errno = 1
			ssec = true
			return
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				m.log.Info(1, "Received stop request, shutting down")
				if err := daemon.Shutdown(context.Background()); err != nil {
					m.log.Error(1, err.Error())
				}
				break loop
			default:
				m.log.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}
