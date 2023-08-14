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
	v1 "github.com/webmeshproj/api/v1"
)

// appID is the application ID.
const appID = "com.webmeshproj.app"

var nodeID = binding.NewString()

// App is the application.
type App struct {
	// App is the fyne application.
	fyne.App
	// main is the main window.
	main fyne.Window
	// cancelMetrics is the cancel function for stopping the metrics updater.
	cancelMetrics context.CancelFunc
	// connecting indicates if the app is currently connecting to the mesh.
	connecting atomic.Bool
	// connected indicates if the app is currently connected to the mesh.
	connected atomic.Bool
	// newCampButton is the button for creating a new campfire.
	newCampButton *widget.Button
	// log is the application logger.
	log *slog.Logger
}

// New sets up and returns a new application.
func New(socketAddr string) *App {
	a := app.NewWithID(appID)
	app := &App{
		App:           a,
		main:          a.NewWindow("WebMesh"),
		newCampButton: widget.NewButton("New Campfire", func() {}),
		log:           slog.Default(),
	}
	if socketAddr != "" {
		app.Preferences().SetString(preferenceNodeSocket, socketAddr)
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

	// Header section
	connectedText := binding.NewString()
	connectedText.Set("Disconnected")
	connectedLabel := widget.NewLabelWithData(connectedText)
	connectSwitch, connected := newConnectSwitch()
	connected.AddListener(binding.NewDataListener(app.onConnectChange(connectedText, connected)))
	campfileEntry := widget.NewEntryWithData(campfireURL)
	campfileEntry.Wrapping = fyne.TextWrapOff
	campfileEntry.SetPlaceHolder("Campfire URI")
	campfileEntry.SetMinRowsVisible(1)
	app.newCampButton = widget.NewButton("New Campfire", app.onNewCampfire)
	app.newCampButton.Alignment = widget.ButtonAlignTrailing
	app.newCampButton.Disable()
	nodeIDWidget := widget.NewLabelWithData(nodeID)
	nodeIDWidget.Alignment = fyne.TextAlignTrailing
	nodeIDWidget.TextStyle = fyne.TextStyle{Italic: true}
	header := container.New(layout.NewHBoxLayout(),
		connectSwitch, connectedLabel, nodeIDWidget,
		layout.NewSpacer(),
		campfileEntry,
		app.newCampButton,
	)

	// Interface metrics section
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
		widget.NewSeparator(),
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
	if app.connected.Load() {
		c, err := app.dialNode()
		if err != nil {
			app.log.Error("error dialing node", "error", err.Error())
			return
		}
		defer c.Close()
		if _, err := v1.NewAppDaemonClient(c).Disconnect(context.Background(), &v1.DisconnectRequest{}); err != nil {
			app.log.Error("error disconnecting from node", "error", err.Error())
		}
	}
}
