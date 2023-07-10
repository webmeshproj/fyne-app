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
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	xlayout "fyne.io/x/fyne/layout"
	"github.com/webmeshproj/node/pkg/ctlcmd/config"
	"github.com/webmeshproj/node/pkg/net/wireguard"
)

const (
	preferenceConfigFile     = "configFile"
	preferenceInterfaceName  = "interfaceName"
	preferenceForceTUN       = "forceTUN"
	preferenceListenPort     = "listenPort"
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
	listenPort     = binding.NewInt()
	raftPort       = binding.NewInt()
	grpcPort       = binding.NewInt()
	disableIPv4    = binding.NewBool()
	disableIPv6    = binding.NewBool()
	connectTimeout = binding.NewInt()
)

// displayPreferences displays the preferences modal.
func (app *App) displayPreferences() {
	form := widget.NewForm(
		widget.NewFormItem("Config file", app.configFileSelector()),
		widget.NewFormItem("Interface", app.interfaceSelector()),
	)
	popup := widget.NewModalPopUp(
		xlayout.NewResponsiveLayout(
			xlayout.Responsive(form, 1, .75, .25),
		),
		app.main.Canvas(),
	)
	form.OnCancel = func() {
		popup.Hide()
	}
	form.OnSubmit = func() {
		popup.Hide()
		// TODO: Save preferences.
	}
	popup.Show()
}

// configFileSelector returns the config file selector.
func (app *App) configFileSelector() fyne.CanvasObject {
	configPath := app.Preferences().StringWithFallback(preferenceConfigFile, config.DefaultConfigPath)
	configFile.Set(configPath)
	entry := widget.NewEntryWithData(configFile)
	entry.Wrapping = fyne.TextWrapOff
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
	return fyne.NewContainerWithLayout(layout.NewHBoxLayout(), entry, dialogSelect)
}

// interfaceSelector returns the interface selector.
func (app *App) interfaceSelector() fyne.CanvasObject {
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
	return fyne.NewContainerWithLayout(layout.NewHBoxLayout(), entry, forceTUNCheck)
}
