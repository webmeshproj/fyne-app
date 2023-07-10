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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/webmeshproj/node/pkg/ctlcmd/config"
	"github.com/webmeshproj/node/pkg/net/wireguard"
)

const (
	preferenceConfigFile     = "configFile"
	preferenceInterfaceName  = "interfaceName"
	preferenceForceTUN       = "forceTUN"
	preferenceWireGuardPort  = "wireguardPort"
	preferenceRaftPort       = "raftPort"
	preferenceGRPCPort       = "grpcPort"
	preferenceDisableIPv4    = "disableIPv4"
	preferenceDisableIPv6    = "disableIPv6"
	preferenceConnectTimeout = "connectTimeout"
)

var (
	configFile     = binding.NewString()
	interfaceName  = binding.NewString()
	forceTUN       = binding.NewBool()
	wireguardPort  = binding.NewString()
	raftPort       = binding.NewString()
	grpcPort       = binding.NewString()
	disableIPv4    = binding.NewBool()
	disableIPv6    = binding.NewBool()
	connectTimeout = binding.NewString()
)

// displayPreferences displays the preferences modal.
func (app *App) displayPreferences() {
	form := widget.NewForm(
		app.configFileFormItem(),
		app.interfaceFormItem(),
		app.portsFormItem(),
		app.timeoutsFormItem(),
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
		configFile, _ := configFile.Get()
		app.Preferences().SetString(preferenceConfigFile, configFile)
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
		// Reload configuration.
		err = app.cli.LoadConfig(configFile)
		if err != nil {
			app.log.Error("error reloading configuration", "error", err.Error())
			err = fmt.Errorf("Configuration file is invalid, try selecting a different one: %w", err)
			dialog.ShowError(err, app.main)
			return
		}
	}
	popup.Show()
}

func (app *App) configFileFormItem() *widget.FormItem {
	configPath := app.Preferences().StringWithFallback(preferenceConfigFile, config.DefaultConfigPath)
	configFile.Set(configPath)
	entry := widget.NewEntryWithData(configFile)
	entry.Wrapping = fyne.TextWrapOff
	entry.Validator = func(s string) error {
		if s == "" {
			return nil
		}
		if s == config.DefaultConfigPath {
			return nil
		}
		_, err := os.Stat(s)
		if err != nil {
			return err
		}
		return nil
	}
	entry.SetPlaceHolder("Webmesh configuration file")
	dialogSelect := widget.NewButton("Open", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				app.log.Error("error opening file", "error", err.Error())
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			selected := reader.URI().String()
			app.log.Info("selected configuration file", "file", selected)
			configFile.Set(strings.TrimPrefix(selected, "file://"))
		}, app.main)
		dir := filepath.Dir(configPath)
		if dir != "" {
			uri := storage.NewFileURI(dir)
			lister, err := storage.ListerForURI(uri)
			if err == nil {
				fileDialog.SetLocation(lister)
			}
		}
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
		fileDialog.Show()
	})
	dialogSelect.SetIcon(theme.FileIcon())
	dialogSelect.Alignment = widget.ButtonAlignTrailing
	formItem := widget.NewFormItem("Config file", fyne.NewContainerWithLayout(layout.NewHBoxLayout(), entry, dialogSelect))
	formItem.HintText = "The path to the WebMesh configuration file."
	return formItem
}

func (app *App) interfaceFormItem() *widget.FormItem {
	interfaceName.Set(app.Preferences().StringWithFallback(preferenceInterfaceName, wireguard.DefaultInterfaceName))
	entry := widget.NewEntryWithData(interfaceName)
	entry.Wrapping = fyne.TextWrapOff
	entry.SetPlaceHolder("Interface name")
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
