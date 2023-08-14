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
	"io"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	v1 "github.com/webmeshproj/api/v1"
)

var (
	connectedInterface = binding.NewString()
	totalSentBytes     = binding.NewString()
	totalRecvBytes     = binding.NewString()
)

func resetConnectedValues() {
	connectedInterface.Set("---")
	totalSentBytes.Set("---")
	totalRecvBytes.Set("---")
}

// onConnectChange fires when the value of the connected switch changes.
func (app *App) onConnectChange(label binding.String, switchValue binding.Float) func() {
	return func() {
		val, err := switchValue.Get()
		if err != nil {
			app.log.Error("error getting connected value", "error", err.Error())
			return
		}
		switch val {
		case switchConnecting:
			// Connect to the mesh if not connected and profile has changed.
			app.connecting.Store(true)
			app.log.Info("connecting to mesh")
			label.Set("Connecting")
			campURL, _ := app.campfireURL.Get()
			var opts v1.ConnectRequest
			if campURL != "" {
				opts.CampfireUri = campURL
			}
			go func() {
				defer app.connecting.Store(false)
				var ctx context.Context
				ctx, app.cancelConnect = context.WithCancel(context.Background())
				resp, err := app.doConnect(ctx, &opts)
				if err != nil {
					if ctx.Err() == nil {
						app.log.Error("error connecting to mesh", "error", err.Error())
						dialog.ShowError(fmt.Errorf("error connecting to mesh: %w", err), app.main)
					}
					label.Set("Disconnected")
					switchValue.Set(switchDisconnected)
					return
				}
				switchValue.Set(switchConnected)
				app.newCampButton.Enable()
				app.connected.Store(true)
				nodeFQDN := fmt.Sprintf("%s.%s", resp.GetNodeId(), resp.GetMeshDomain())
				app.nodeID.Set(fmt.Sprintf("Connected as %q", nodeFQDN))
				app.chatContainer.Show()
			}()
		case switchConnected:
			label.Set("Connected")
			ctx := context.Background()
			ctx, app.cancelNodeSubscriptions = context.WithCancel(ctx)
			c, err := app.dialNode(ctx)
			if err != nil {
				app.log.Error("error dialing node", "error", err.Error())
			} else {
				// Subscribe to new rooms as they come in
				go func() {
					app.log.Info("subscribing to new rooms")
					stream, err := v1.NewAppDaemonClient(c).Subscribe(ctx, &v1.SubscribeRequest{
						Prefix: RoomsPrefix,
					})
					if err != nil {
						app.log.Error("error subscribing to rooms", "error", err.Error())
						return
					}
					defer stream.CloseSend()
					for {
						resp, err := stream.Recv()
						if err != nil {
							if err == context.Canceled || err == io.EOF {
								return
							}
							app.log.Error("error receiving room", "error", err.Error())
							return
						}
						prefix := strings.TrimPrefix(resp.GetKey(), RoomsPrefix+"/")
						parts := strings.Split(prefix, "/")
						if len(parts) == 1 {
							app.roomsList.Append(parts[0])
						}
					}
				}()
			}
			// Try to fetch the current list of rooms.
			rooms, err := app.listRooms()
			if err != nil {
				app.log.Error("error listing rooms", "error", err.Error())
			} else {
				app.roomsList.Set(rooms)
			}
			metrics, err := app.getNodeMetrics(ctx)
			if err != nil {
				app.log.Error("error getting interface metrics", "error", err.Error())
			} else {
				connectedInterface.Set(metrics.DeviceName)
			}

			go func() {
				t := time.NewTicker(time.Second * 5)
				defer t.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-t.C:
						metrics, err := app.getNodeMetrics(ctx)
						if err != nil {
							app.log.Error("error getting interface metrics", "error", err.Error())
							continue
						}
						totalSentBytes.Set(bytesString(int(metrics.TotalTransmitBytes)))
						totalRecvBytes.Set(bytesString(int(metrics.TotalReceiveBytes)))
					}
				}
			}()
		case switchDisconnected:
			// Disconnect from the mesh.
			defer resetConnectedValues()
			defer app.cancelNodeSubscriptions()
			if app.connecting.Load() {
				app.log.Info("cancelling in-progress connection")
				app.cancelConnect()
				return
			}
			if !app.connected.Load() {
				return
			}
			app.log.Info("disconnecting from mesh")
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				err := app.doDisconnect(ctx)
				if err != nil {
					if !strings.Contains(err.Error(), "not connected") {
						app.log.Error("error disconnecting from mesh", "error", err.Error())
						dialog.ShowError(fmt.Errorf("error disconnecting from mesh: %w", err), app.main)
					}
				}
				label.Set("Disconnected")
				app.newCampButton.Disable()
				app.campfireURL.Set("")
				app.connected.Store(false)
				app.nodeID.Set("")
				app.chatContainer.Hide()
				app.chatTextGrid.SetText("")
				app.roomsList.Set([]string{})
			}()
		}
	}
}

func bytesString(n int) string {
	if n < 1024 {
		return strconv.Itoa(n) + " B"
	} else if n < 1024*1024 {
		return strconv.Itoa(n/1024) + " KB"
	} else if n < 1024*1024*1024 {
		return strconv.Itoa(n/1024/1024) + " MB"
	}
	return strconv.Itoa(n/1024/1024/1024) + " GB"
}
