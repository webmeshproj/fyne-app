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

package app

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/webmeshproj/api/v1"
	"github.com/webmeshproj/webmesh/pkg/campfire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (app *App) doConnect(opts *v1.ConnectRequest) (*v1.ConnectResponse, error) {
	c, err := app.dialNode()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return v1.NewAppDaemonClient(c).Connect(context.Background(), opts)
}

func (app *App) doDisconnect() error {
	c, err := app.dialNode()
	if err != nil {
		return err
	}
	defer c.Close()
	cli := v1.NewAppDaemonClient(c)
	_, err = cli.Disconnect(context.Background(), &v1.DisconnectRequest{})
	return err
}

func (app *App) getNodeMetrics() (*v1.InterfaceMetrics, error) {
	c, err := app.dialNode()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	resp, err := v1.NewAppDaemonClient(c).Metrics(context.Background(), &v1.MetricsRequest{})
	if err != nil {
		return nil, err
	}
	for _, m := range resp.GetInterfaces() {
		return m, nil
	}
	return nil, fmt.Errorf("no metrics returned")
}

func (app *App) startCampfire(uri *campfire.CampfireURI) error {
	c, err := app.dialNode()
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = v1.NewAppDaemonClient(c).StartCampfire(context.Background(), &v1.StartCampfireRequest{
		CampUrl: uri.EncodeURI(),
	})
	return err
}

func (app *App) dialNode() (*grpc.ClientConn, error) {
	socket := app.Preferences().StringWithFallback(preferenceNodeSocket, "tcp://127.0.0.1:8080")
	socket = strings.TrimPrefix(socket, "tcp://")
	c, err := grpc.Dial(socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		app.log.Error("failed to connect to node", "error", err.Error())
		return nil, err
	}
	return c, nil
}
