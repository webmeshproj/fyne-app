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
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	v1 "github.com/webmeshproj/api/v1"
	"github.com/webmeshproj/webmesh/pkg/campfire"
	"google.golang.org/protobuf/types/known/structpb"
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
			campURL, _ := campfireURL.Get()
			connectCfg := make(map[string]any)
			if campURL != "" {
				parsed, err := campfire.ParseCampfireURI(campURL)
				if err != nil {
					app.log.Error("error parsing campfire url", "error", err.Error())
					dialog.ShowError(fmt.Errorf("invaid Campfire URL"), app.main)
					return
				}
				connectCfg["mesh"] = map[string]any{
					"join-campfire-psk":          string(parsed.PSK),
					"join-campfire-turn-servers": parsed.TURNServers,
				}
				connectCfg["bootstrap"] = map[string]any{
					"enabled": false,
				}
			}
			var opts v1.ConnectRequest
			var err error
			opts.Config, err = structpb.NewStruct(connectCfg)
			if err != nil {
				app.log.Error("error creating connect config", "error", err.Error())
				dialog.ShowError(fmt.Errorf("error creating connect config: %w", err), app.main)
				return
			}
			go func() {
				defer app.connecting.Store(false)
				resp, err := app.doConnect(&opts)
				if err != nil {
					app.log.Error("error connecting to mesh", "error", err.Error())
					dialog.ShowError(fmt.Errorf("error connecting to mesh: %w", err), app.main)
					label.Set("Disconnected")
					switchValue.Set(switchDisconnected)
					return
				}
				switchValue.Set(switchConnected)
				app.newCampButton.Enable()
				app.connected.Store(true)
				nodeFQDN := fmt.Sprintf("%s.%s", resp.GetNodeId(), resp.GetMeshDomain())
				nodeID.Set(fmt.Sprintf("Connected as %q", nodeFQDN))
			}()
		case switchConnected:
			label.Set("Connected")
			ctx := context.Background()
			metrics, err := app.getNodeMetrics()
			if err != nil {
				app.log.Error("error getting interface metrics", "error", err.Error())
				return
			}
			connectedInterface.Set(metrics.DeviceName)
			ctx, app.cancelMetrics = context.WithCancel(ctx)
			go func() {
				t := time.NewTicker(time.Second * 5)
				defer t.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-t.C:
						metrics, err := app.getNodeMetrics()
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
			if app.cancelMetrics != nil {
				app.cancelMetrics()
			}
			defer resetConnectedValues()
			if !app.connected.Load() {
				return
			}
			app.log.Info("disconnecting from mesh")
			if app.connecting.Load() {
				app.log.Info("cancelling in-progress connection")
				// app.cli.CancelConnect() // TODO: Implement.
			}
			go func() {
				err := app.doDisconnect()
				if err != nil {
					if !strings.Contains(err.Error(), "not connected") {
						app.log.Error("error disconnecting from mesh", "error", err.Error())
						dialog.ShowError(fmt.Errorf("error disconnecting from mesh: %w", err), app.main)
					}
				}
				app.newCampButton.Disable()
				label.Set("Disconnected")
				campfireURL.Set("")
				app.connected.Store(false)
				nodeID.Set("")
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
