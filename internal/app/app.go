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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	v1 "github.com/webmeshproj/api/v1"
)

const (
	// AppID is the application ID.
	AppID = "com.webmeshproj.app"
)

// App is the application.
type App struct {
	// App is the fyne application.
	fyne.App
	// main is the main window.
	main fyne.Window
	// nodeID is the ID of the node.
	nodeID binding.String
	// nodeIDDisplay is the display for the node ID.
	nodeIDDisplay binding.String
	// campfireURL is the current campfire URL.
	campfireURL binding.String
	// cancelNodeSubscriptions is the cancel function for stopping the node subscriptions.
	cancelNodeSubscriptions context.CancelFunc
	// cancelRoomSubscription is the cancel function for stopping the room subscription.
	cancelRoomSubscription context.CancelFunc
	// cancelConnect is the cancel function for stopping the an in-progress connection.
	cancelConnect context.CancelFunc
	// connecting indicates if the app is currently connecting to the mesh.
	connecting atomic.Bool
	// connected indicates if the app is currently connected to the mesh.
	connected atomic.Bool
	// newCampButton is the button for creating a new campfire.
	newCampButton *widget.Button
	// roomsList is the list of rooms.
	roomsList binding.StringList
	// roomsListWidget is the widget containing the list of rooms.
	roomsListWidget *widget.List
	// chatContainer is the container for the chat room.
	chatContainer *fyne.Container
	// chatText is the grid for the chat text.
	chatText *widget.TextGrid
	// chatGrid is the container containg the chat text and input.
	chatGrid *fyne.Container
	// chatInput is the input for the chat.
	chatInput *widget.Entry
	// joinRooms is the list of joined rooms.
	joinRooms []string
	// selectedRoom is the currently selected room.
	selectedRoom string
	// log is the application logger.
	log *slog.Logger
}

// New sets up and returns a new application.
func New(socketAddr string) *App {
	a := app.NewWithID(AppID)
	app := &App{
		App:                     a,
		main:                    a.NewWindow("Webmesh Campfire"),
		nodeID:                  binding.NewString(),
		nodeIDDisplay:           binding.NewString(),
		campfireURL:             binding.NewString(),
		newCampButton:           widget.NewButton("New Campfire", func() {}),
		roomsList:               binding.NewStringList(),
		chatText:                widget.NewTextGrid(),
		chatInput:               widget.NewEntry(),
		cancelNodeSubscriptions: func() {},
		cancelConnect:           func() {},
		log:                     slog.Default(),
	}
	if socketAddr != "" {
		nodeSocket.Set(socketAddr)
	} else {
		nodeSocket.Set(app.Preferences().StringWithFallback(preferenceNodeSocket, "tcp://127.0.0.1:8080"))
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
	campfileEntry := widget.NewEntryWithData(app.campfireURL)
	campfileEntry.Wrapping = fyne.TextWrapOff
	campfileEntry.SetPlaceHolder("Campfire URI")
	campfileEntry.SetMinRowsVisible(1)
	campfileEntry.OnChanged = func(s string) {
		app.campfireURL.Set(s)
	}
	app.newCampButton = widget.NewButton("New Campfire", app.onNewCampfire)
	app.newCampButton.Alignment = widget.ButtonAlignTrailing
	app.newCampButton.Disable()
	nodeIDWidget := widget.NewLabelWithData(app.nodeIDDisplay)
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

	// Chat rooms
	newRoomLabel := func() fyne.CanvasObject { return widget.NewLabel("") }
	renderRoom := func(item binding.DataItem, obj fyne.CanvasObject) {
		obj.(*widget.Label).Bind(item.(binding.String))
	}
	app.roomsListWidget = widget.NewListWithData(app.roomsList, newRoomLabel, renderRoom)
	app.roomsListWidget.OnSelected = app.onRoomSelected
	app.roomsListWidget.OnUnselected = app.onRoomUnselected
	roomsTop := container.New(layout.NewVBoxLayout(),
		widget.NewButton("New Room", app.onNewChatRoom),
		widget.NewLabel("Chat Rooms"))
	roomsContainer := container.New(layout.NewBorderLayout(roomsTop, nil, nil, nil),
		roomsTop,
		app.roomsListWidget,
	)
	roomBox := container.New(layout.NewHBoxLayout(), roomsContainer, widget.NewSeparator())
	app.chatInput.SetPlaceHolder("Enter message")
	app.chatInput.OnSubmitted = app.onSendMessage
	app.chatInput.Wrapping = fyne.TextWrapWord
	app.chatGrid = container.New(layout.NewBorderLayout(nil, app.chatInput, nil, nil), app.chatText, app.chatInput)
	app.chatContainer = container.New(layout.NewBorderLayout(nil, nil, roomBox, nil),
		roomBox,
		app.chatGrid,
	)
	app.chatGrid.Hide()
	app.chatContainer.Hide()

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
	top := container.New(layout.NewVBoxLayout(), header, body)
	app.main.SetContent(container.New(layout.NewBorderLayout(top, nil, nil, nil),
		top,
		app.chatContainer,
	))
}

// closeIntercept is fired before the main window is closed.
func (app *App) closeIntercept() {
	defer app.main.Close()
	if app.connected.Load() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		c, err := app.dialNode(ctx)
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
