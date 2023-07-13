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
	"runtime"
	"strconv"
	"time"

	"fyne.io/fyne/v2/data/binding"
	"github.com/webmeshproj/node/pkg/net/wireguard"

	"github.com/webmeshproj/app/internal/daemon"
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
			profile, err := app.currentProfile.Get()
			if err != nil {
				app.log.Error("error getting profile", "error", err.Error())
				// TODO: Display error.
				switchValue.Set(switchDisconnected)
				return
			} else if profile == "" || profile == noProfiles {
				app.log.Info("current configuration has no profiles")
				switchValue.Set(switchDisconnected)
				return
			}
			app.log.Info("connecting to mesh", "profile", profile)
			label.Set("Connecting")
			requiresTUN := runtime.GOOS != "linux" && runtime.GOOS != "freebsd"
			go func() {
				err = app.cli.Connect(context.Background(), daemon.ConnectOptions{
					Profile:       profile,
					InterfaceName: app.Preferences().StringWithFallback(preferenceInterfaceName, wireguard.DefaultInterfaceName),
					ForceTUN:      app.Preferences().BoolWithFallback(preferenceForceTUN, requiresTUN),
					ListenPort: func() uint16 {
						v, _ := strconv.ParseUint(app.Preferences().StringWithFallback(preferenceWireGuardPort, "51820"), 10, 16)
						return uint16(v)
					}(),
					RaftPort: func() uint16 {
						v, _ := strconv.ParseUint(app.Preferences().StringWithFallback(preferenceRaftPort, "9443"), 10, 16)
						return uint16(v)
					}(),
					GRPCPort: func() uint16 {
						v, _ := strconv.ParseUint(app.Preferences().StringWithFallback(preferenceGRPCPort, "8443"), 10, 16)
						return uint16(v)
					}(),
					NoIPv4: app.Preferences().BoolWithFallback(preferenceDisableIPv4, false),
					NoIPv6: app.Preferences().BoolWithFallback(preferenceDisableIPv6, false),
					ConnectTimeout: func() int {
						d, _ := time.ParseDuration(app.Preferences().StringWithFallback(preferenceConnectTimeout, "30s"))
						return int(d.Seconds())
					}(),
					// TODO:
					LocalDNS:     false,
					LocalDNSPort: 0,
				})
				if err != nil {
					app.log.Error("error connecting to mesh", "error", err.Error())
					// TODO: Display error.
					label.Set("Disconnected")
					switchValue.Set(switchDisconnected)
					return
				}
				switchValue.Set(switchConnected)
			}()
		case switchConnected:
			label.Set("Connected")
			ctx := context.Background()
			metrics, err := app.cli.InterfaceMetrics(ctx)
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
						metrics, err = app.cli.InterfaceMetrics(ctx)
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
			app.log.Info("disconnecting from mesh")
			if app.cli.Connecting() {
				app.log.Info("cancelling in-progress connection")
				app.cli.CancelConnect()
			}
			go func() {
				err := app.cli.Disconnect(context.Background())
				if err != nil && !daemon.IsNotConnected(err) {
					app.log.Error("error disconnecting from mesh", "error", err.Error())
					// Handle the error.
				}
				label.Set("Disconnected")
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
