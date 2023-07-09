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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/webmeshproj/node/pkg/ctlcmd/config"
	"golang.org/x/exp/slog"

	"github.com/webmeshproj/app/internal/daemon"
)

// appID is the application ID.
const appID = "com.webmeshproj.app"

// App is the application.
type App struct {
	fyne.App
	// main is the main window.
	main fyne.Window
	// cli is the daemon client. It is used to communicate with the daemon.
	// When running as root on unix-like systems and no daemon is available,
	// this will be a pass-through client that executes the requested command
	// directly.
	cli daemon.Client
}

// New sets up and returns a new application.
func New() *App {
	a := app.NewWithID(appID)
	app := &App{
		App:  a,
		main: a.NewWindow("WebMesh"),
		cli:  daemon.NewClient(),
	}
	// See if we are able to load the config file.
	err := app.cli.LoadConfig(func() string {
		return app.Preferences().StringWithFallback("config-file", config.DefaultConfigPath)
	}())
	if err != nil {
		slog.Default().Error("error loading config", "error", err.Error())
	}
	// Set a close interceptor to make sure we disconnect on shutdown.
	app.main.SetCloseIntercept(func() {
		defer app.main.Close()
		if app.cli.Connected() {
			err := app.cli.Disconnect(context.Background())
			if err != nil {
				slog.Default().Error("error disconnecting from mesh", "error", err.Error())
			}
		}
	})
	app.setupCanvas()
	app.main.Resize(fyne.NewSize(400, 300))
	app.main.Show()
	return app
}

// setupCanvas sets up the initial state of the main canvas.
func (app *App) setupCanvas() {
	connectedText := binding.NewString()
	connectedText.Set("Disconnected")
	connectedLabel := widget.NewLabelWithData(connectedText)
	connectSwitch, connected := newConnectSwitch()
	connected.AddListener(binding.NewDataListener(func() {
		val, _ := connected.Get()
		switch val {
		case switchConnecting, switchConnected:
			// Connect to the mesh if not connected and profile has changed.
			connectSwitch.SetValue(switchConnected)
			connectedText.Set("Connected")
		case switchDisconnected:
			// Disconnect from the mesh.
			connectedText.Set("Disconnected")
		}
	}))
	header := container.New(layout.NewHBoxLayout(), connectSwitch, connectedLabel, layout.NewSpacer())
	app.main.SetContent(container.New(layout.NewVBoxLayout(),
		header,
		widget.NewSeparator(),
	))
}
