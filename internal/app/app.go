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
	// App is the fyne application.
	fyne.App
	// main is the main window.
	main fyne.Window
	// cli is the daemon client. It is used to communicate with the daemon.
	// When running as root on unix-like systems and no daemon is available,
	// this will be a pass-through client that executes the requested command
	// directly.
	cli daemon.Client
	// log is the application logger.
	log *slog.Logger
}

// New sets up and returns a new application.
func New() *App {
	a := app.NewWithID(appID)
	app := &App{
		App:  a,
		main: a.NewWindow("WebMesh"),
		cli:  daemon.NewClient(),
		log:  slog.Default(),
	}
	app.setup()
	app.main.Show()
	return app
}

// setupMain sets up the initial state of the app.
func (app *App) setup() {
	err := app.cli.LoadConfig(func() string {
		return app.Preferences().StringWithFallback(preferenceConfigFile, config.DefaultConfigPath)
	}())
	if err != nil {
		app.log.Error("error loading config", "error", err.Error())
	}
	app.main.Resize(fyne.NewSize(800, 600))
	app.main.SetCloseIntercept(app.closeIntercept)
	app.main.SetMainMenu(app.newMainMenu())
	connectedText := binding.NewString()
	connectedText.Set("Disconnected")
	connectedLabel := widget.NewLabelWithData(connectedText)
	connectSwitch, connected := newConnectSwitch()
	connected.AddListener(binding.NewDataListener(app.onConnectChange(connectedText, connected)))
	header := container.New(layout.NewHBoxLayout(), connectSwitch, connectedLabel, layout.NewSpacer())
	app.main.SetContent(container.New(layout.NewVBoxLayout(),
		header,
		widget.NewSeparator(),
	))
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
			app.log.Info("connecting to mesh")
			label.Set("Connecting")
			switchValue.Set(switchConnected)
		case switchConnected:
			label.Set("Connected")
		case switchDisconnected:
			// Disconnect from the mesh.
			app.log.Info("disconnecting from mesh")
			err := app.cli.Disconnect(context.Background())
			if err != nil && !daemon.IsNotConnected(err) {
				app.log.Error("error disconnecting from mesh", "error", err.Error())
				// Handle the error.
			}
			label.Set("Disconnected")
		}
	}
}

// closeIntercept is fired before the main window is closed.
func (app *App) closeIntercept() {
	defer app.main.Close()
	if app.cli.Connected() {
		err := app.cli.Disconnect(context.Background())
		if err != nil {
			app.log.Error("error disconnecting from mesh", "error", err.Error())
		}
	}
}
