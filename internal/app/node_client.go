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

func (app *App) doConnect(ctx context.Context, opts *v1.ConnectRequest) (*v1.ConnectResponse, error) {
	c, err := app.dialNode(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return v1.NewAppDaemonClient(c).Connect(ctx, opts)
}

func (app *App) doDisconnect(ctx context.Context) error {
	c, err := app.dialNode(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	cli := v1.NewAppDaemonClient(c)
	_, err = cli.Disconnect(ctx, &v1.DisconnectRequest{})
	return err
}

func (app *App) getNodeMetrics(ctx context.Context) (*v1.InterfaceMetrics, error) {
	c, err := app.dialNode(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	resp, err := v1.NewAppDaemonClient(c).Metrics(ctx, &v1.MetricsRequest{})
	if err != nil {
		return nil, err
	}
	for _, m := range resp.GetInterfaces() {
		return m, nil
	}
	return nil, fmt.Errorf("no metrics returned")
}

func (app *App) startCampfire(ctx context.Context, uri *campfire.CampfireURI) error {
	c, err := app.dialNode(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = v1.NewAppDaemonClient(c).StartCampfire(ctx, &v1.StartCampfireRequest{
		CampUrl: uri.EncodeURI(),
	})
	return err
}

func (app *App) dialNode(ctx context.Context) (*grpc.ClientConn, error) {
	socketAddr, err := nodeSocket.Get()
	if err != nil {
		return nil, err
	}
	socket := strings.TrimPrefix(socketAddr, "tcp://")
	c, err := grpc.DialContext(ctx, socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		app.log.Error("failed to connect to node", "error", err.Error())
		return nil, err
	}
	return c, nil
}
