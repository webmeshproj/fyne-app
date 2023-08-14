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
	"log/slog"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// appID is the application ID.
const appID = "com.webmeshproj.app"

// App is the application.
type App struct {
	// App is the fyne application.
	fyne.App
	// main is the main window.
	main fyne.Window
	// currentProfile is the current profile.
	currentProfile binding.String
	// cancelMetrics is the cancel function for stopping the metrics updater.
	cancelMetrics context.CancelFunc
	// connecting indicates if the app is currently connecting to the mesh.
	connecting atomic.Bool
	// log is the application logger.
	log *slog.Logger
}

// New sets up and returns a new application.
func New() *App {
	a := app.NewWithID(appID)
	app := &App{
		App:            a,
		main:           a.NewWindow("WebMesh"),
		currentProfile: binding.NewString(),
		log:            slog.Default(),
	}
	app.setup()
	app.main.Show()
	return app
}

// setupMain sets up the initial state of the app.
func (app *App) setup() {
	app.main.Resize(fyne.NewSize(800, 600))
	app.main.SetCloseIntercept(app.closeIntercept)
	app.main.SetMainMenu(app.newMainMenu())
	connectedText := binding.NewString()
	connectedText.Set("Disconnected")
	connectedLabel := widget.NewLabelWithData(connectedText)
	connectSwitch, connected := newConnectSwitch()
	connected.AddListener(binding.NewDataListener(app.onConnectChange(connectedText, connected)))
	header := container.New(layout.NewHBoxLayout(),
		connectSwitch, connectedLabel,
		layout.NewSpacer(),
	)
	ifaceLabel := widget.NewLabel("Interface")
	sentLabel := widget.NewLabel("Total Sent")
	rcvdLabel := widget.NewLabel("Total Received")
	ifaceLabel.TextStyle.Bold = true
	sentLabel.TextStyle.Bold = true
	rcvdLabel.TextStyle.Bold = true
	body := container.New(layout.NewVBoxLayout(),
		container.New(layout.NewHBoxLayout(),
			ifaceLabel, widget.NewLabelWithData(connectedInterface), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(),
			sentLabel, widget.NewLabelWithData(totalSentBytes), layout.NewSpacer()),
		container.New(layout.NewHBoxLayout(),
			rcvdLabel, widget.NewLabelWithData(totalRecvBytes), layout.NewSpacer()),
	)
	resetConnectedValues()
	app.main.SetContent(container.New(layout.NewVBoxLayout(),
		header,
		widget.NewSeparator(),
		body,
	))
}

// closeIntercept is fired before the main window is closed.
func (app *App) closeIntercept() {
	defer app.main.Close()
	// if app.cli.Connected() {
	// 	err := app.cli.Disconnect(context.Background())
	// 	if err != nil {
	// 		app.log.Error("error disconnecting from mesh", "error", err.Error())
	// 	}
	// }
}
