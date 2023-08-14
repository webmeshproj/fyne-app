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
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/webmeshproj/webmesh/pkg/net/wireguard"
)

const (
	preferenceInterfaceName  = "interfaceName"
	preferenceForceTUN       = "forceTUN"
	preferenceWireGuardPort  = "wireguardPort"
	preferenceRaftPort       = "raftPort"
	preferenceGRPCPort       = "grpcPort"
	preferenceDisableIPv4    = "disableIPv4"
	preferenceDisableIPv6    = "disableIPv6"
	preferenceConnectTimeout = "connectTimeout"
	preferenceNodeSocket     = "nodeSocket"
	preferenceTURNServers    = "turnServers"
)

var (
	interfaceName  = binding.NewString()
	forceTUN       = binding.NewBool()
	wireguardPort  = binding.NewString()
	raftPort       = binding.NewString()
	grpcPort       = binding.NewString()
	disableIPv4    = binding.NewBool()
	disableIPv6    = binding.NewBool()
	connectTimeout = binding.NewString()
	nodeSocket     = binding.NewString()
	turnServers    = binding.NewString()
)

// displayPreferences displays the preferences modal.
func (app *App) displayPreferences() {
	form := widget.NewForm(
		app.socketFormItem(),
		app.interfaceFormItem(),
		app.portsFormItem(),
		app.timeoutsFormItem(),
		app.turnServersFormItem(),
		app.protocolFormItem(),
	)
	popup := widget.NewModalPopUp(
		form,
		app.main.Canvas(),
	)
	form.OnCancel = func() {
		popup.Hide()
	}
	form.OnSubmit = func() {
		err := validatePreferences()
		if err != nil {
			app.log.Error("error validating preferences", "error", err.Error())
			dialog.ShowError(err, app.main)
			return
		}
		defer popup.Hide()
		// Save preferences.
		nodeSocket, _ := nodeSocket.Get()
		app.Preferences().SetString(preferenceNodeSocket, nodeSocket)
		interfaceName, _ := interfaceName.Get()
		app.Preferences().SetString(preferenceInterfaceName, interfaceName)
		forceTUN, _ := forceTUN.Get()
		app.Preferences().SetBool(preferenceForceTUN, forceTUN)
		wireguardPort, _ := wireguardPort.Get()
		app.Preferences().SetString(preferenceWireGuardPort, wireguardPort)
		raftPort, _ := raftPort.Get()
		app.Preferences().SetString(preferenceRaftPort, raftPort)
		grpcPort, _ := grpcPort.Get()
		app.Preferences().SetString(preferenceGRPCPort, grpcPort)
		disableIPv4, _ := disableIPv4.Get()
		app.Preferences().SetBool(preferenceDisableIPv4, disableIPv4)
		disableIPv6, _ := disableIPv6.Get()
		app.Preferences().SetBool(preferenceDisableIPv6, disableIPv6)
		connectTimeout, _ := connectTimeout.Get()
		app.Preferences().SetString(preferenceConnectTimeout, connectTimeout)
		turnServers, _ := turnServers.Get()
		app.Preferences().SetString(preferenceTURNServers, strings.Replace(turnServers, "\n", ",", -1))
	}
	popup.Show()
}

func (app *App) socketFormItem() *widget.FormItem {
	socket := app.Preferences().StringWithFallback(preferenceNodeSocket, "tcp://127.0.0.1:8080")
	nodeSocket.Set(socket)
	nodeSocketInput := widget.NewEntryWithData(nodeSocket)
	nodeSocketInput.Wrapping = fyne.TextWrapOff
	nodeSocketInput.OnChanged = func(s string) {
		app.Preferences().SetString(preferenceNodeSocket, s)
	}
	formItem := widget.NewFormItem("Node Socket", nodeSocketInput)
	formItem.HintText = "The socket to use to connect to the node."
	return formItem
}

func (app *App) interfaceFormItem() *widget.FormItem {
	interfaceName.Set(app.Preferences().StringWithFallback(preferenceInterfaceName, wireguard.DefaultInterfaceName))
	entry := widget.NewEntryWithData(interfaceName)
	entry.Wrapping = fyne.TextWrapOff
	entry.SetPlaceHolder("Interface name")
	if runtime.GOOS == "darwin" {
		// This is immutable on macOS.
		entry.Disable()
	}
	// Kernel interface is only supported on Linux and FreeBSD.
	requiresTUN := runtime.GOOS != "linux" && runtime.GOOS != "freebsd"
	forceTUNCheck := widget.NewCheckWithData("Force TUN", forceTUN)
	forceTUNCheck.SetChecked(app.Preferences().BoolWithFallback(preferenceForceTUN, requiresTUN))
	if requiresTUN {
		forceTUNCheck.Disable()
	}
	formItem := widget.NewFormItem("Interface", fyne.NewContainerWithLayout(layout.NewHBoxLayout(), entry, forceTUNCheck))
	formItem.HintText = "The name and type of the interface to use for the mesh."
	return formItem
}

func (app *App) portsFormItem() *widget.FormItem {
	wireguardPort.Set(app.Preferences().StringWithFallback(preferenceWireGuardPort, "51820"))
	grpcPort.Set(app.Preferences().StringWithFallback(preferenceGRPCPort, "8443"))
	raftPort.Set(app.Preferences().StringWithFallback(preferenceRaftPort, "9443"))
	isValidPort := func(s string) error {
		_, err := strconv.ParseUint(s, 10, 16)
		return err
	}
	wireguardEntry := widget.NewEntryWithData(wireguardPort)
	wireguardEntry.Wrapping = fyne.TextWrapOff
	wireguardEntry.Validator = isValidPort
	wireguardEntry.SetPlaceHolder("WireGuard port")
	grpcEntry := widget.NewEntryWithData(grpcPort)
	grpcEntry.Wrapping = fyne.TextWrapOff
	grpcEntry.Validator = isValidPort
	grpcEntry.SetPlaceHolder("gRPC port")
	raftEntry := widget.NewEntryWithData(raftPort)
	raftEntry.Wrapping = fyne.TextWrapOff
	raftEntry.Validator = isValidPort
	raftEntry.SetPlaceHolder("Raft port")
	formItem := widget.NewFormItem("Ports", fyne.NewContainerWithLayout(layout.NewHBoxLayout(),
		widget.NewLabel("WireGuard"), wireguardEntry,
		widget.NewLabel("gRPC"), grpcEntry,
		widget.NewLabel("Raft"), raftEntry,
	))
	formItem.HintText = "Ports for inter-node communication"
	return formItem
}

func (app *App) timeoutsFormItem() *widget.FormItem {
	connectTimeout.Set(app.Preferences().StringWithFallback(preferenceConnectTimeout, "30s"))
	connectTimeoutEntry := widget.NewEntryWithData(connectTimeout)
	connectTimeoutEntry.Wrapping = fyne.TextWrapOff
	connectTimeoutEntry.SetPlaceHolder("Connect timeout")
	connectTimeoutEntry.Validator = func(s string) error {
		_, err := time.ParseDuration(s)
		return err
	}
	formItem := widget.NewFormItem("Timeouts", fyne.NewContainerWithLayout(layout.NewHBoxLayout(),
		widget.NewLabel("Connect timeout"), connectTimeoutEntry,
	))
	formItem.HintText = "Timeouts for connecting to the mesh"
	return formItem
}

func (app *App) protocolFormItem() *widget.FormItem {
	disableIPv4.Set(app.Preferences().BoolWithFallback(preferenceDisableIPv4, false))
	disableIPv6.Set(app.Preferences().BoolWithFallback(preferenceDisableIPv6, false))
	ipv4Check := widget.NewCheckWithData("Disable IPv4", disableIPv4)
	ipv6Check := widget.NewCheckWithData("Disable IPv6", disableIPv6)
	formItem := widget.NewFormItem("Protocol", fyne.NewContainerWithLayout(layout.NewHBoxLayout(), ipv4Check, ipv6Check))
	formItem.HintText = "Protocol options for the mesh"
	return formItem
}

func (app *App) turnServersFormItem() *widget.FormItem {
	turnServerPreferences := app.Preferences().StringWithFallback(preferenceTURNServers, "")
	var turnServerStrs []string
	if turnServerPreferences != "" {
		turnServerStrs = strings.Split(turnServerPreferences, ",")
	}
	turnServers.Set(strings.Join(turnServerStrs, "\n"))
	list := widget.NewEntryWithData(turnServers)
	list.MultiLine = true
	list.PlaceHolder = "turn:example.com:3478"
	formItem := widget.NewFormItem("TURN Servers", list)
	formItem.HintText = "Newline separated list of TURN servers to use for NAT traversal"
	return formItem
}
