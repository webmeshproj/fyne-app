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
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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
