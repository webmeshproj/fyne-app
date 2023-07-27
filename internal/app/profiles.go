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
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/webmeshproj/webmesh/pkg/ctlcmd/config"
)

const noProfiles = "No profiles"

func (app *App) reloadProfileSelector() {
	selected := ""
	config := app.cli.Config()
	if len(config.Contexts) == 0 {
		selected = noProfiles
	} else if config.CurrentContext != "" {
		selected = config.CurrentContext
	} else {
		selected = config.Contexts[0].Name
	}
	profiles := app.profileOptions()
	app.profiles.Options = profiles
	app.profiles.Selected = selected
	app.profiles.OnChanged = func(selected string) {
		if len(profiles) == 1 && profiles[0] == noProfiles {
			return
		}
		// TODO: If already connected to a profile, prompt if okay to switch connections
		// and reconnect.
		app.currentProfile.Set(selected)
	}
	// If a profile is not already selected, store the current one.
	current, err := app.currentProfile.Get()
	if err != nil || (current == "" || current == noProfiles) {
		app.profiles.SetSelected(selected)
	}
}

func (app *App) onAddProfile() {
	app.showProfileEditor("", true)
}

func (app *App) onEditProfile() {
	current, _ := app.currentProfile.Get()
	if current == "" || current == noProfiles {
		app.log.Info("no profile selected to edit")
		return
	}
	app.showProfileEditor(current, false)
}

func (app *App) showProfileEditor(name string, isNew bool) {
	title := name
	currentConfig := app.cli.Config()
	if isNew {
		title = "New Profile"
	}

	// Bindings
	profileName := binding.NewString()
	serverAddress := binding.NewString()
	insecure := binding.NewBool()
	verifyChainOnly := binding.NewBool()
	skipVerify := binding.NewBool()
	caData := binding.NewString()
	currentAuthMethod := binding.NewString()
	username := binding.NewString()
	password := binding.NewString()
	certData := binding.NewString()
	keyData := binding.NewString()

	// Profile Name
	profileName.Set(name)
	nameEntry := widget.NewEntryWithData(profileName)
	nameEntry.SetPlaceHolder("Enter a name for this profile")
	nameEntry.Wrapping = fyne.TextWrapOff
	nameEntry.Validator = func(s string) error {
		if s == "" {
			return errors.New("name is required")
		}
		if isNew {
			for _, ctx := range currentConfig.Contexts {
				if ctx.Name == s {
					return fmt.Errorf("profile with name %q already exists", s)
				}
			}
		}
		return nil
	}
	if !isNew {
		nameEntry.Disable()
	}
	nameFormItem := widget.NewFormItem("Name", nameEntry)

	// Server Address
	if !isNew {
		profile := currentConfig.GetContext(name)
		cluster := currentConfig.GetCluster(profile.Cluster)
		serverAddress.Set(cluster.Server)
	}
	serverEntry := widget.NewEntryWithData(serverAddress)
	serverEntry.SetPlaceHolder("Enter the server address")
	serverEntry.Wrapping = fyne.TextWrapOff
	serverEntry.Validator = func(s string) error {
		if s == "" {
			return errors.New("server address is required")
		}
		// Fake a scheme so we can use url.ParseRequestURI.
		u, err := url.ParseRequestURI("http://" + s)
		if err != nil {
			return err
		}
		if u.Port() == "" {
			return errors.New("server address should include a port")
		}
		if u.Path != "" || u.RawQuery != "" {
			return errors.New("server address should only include a host and port")
		}
		return nil
	}
	serverFormItem := widget.NewFormItem("Server Address", serverEntry)

	// Transport Security
	selectedTransport := ""
	if !isNew {
		profile := currentConfig.GetContext(name)
		cluster := currentConfig.GetCluster(profile.Cluster)
		insecure.Set(cluster.Insecure)
		verifyChainOnly.Set(cluster.TLSVerifyChainOnly)
		skipVerify.Set(cluster.TLSSkipVerify)
		if cluster.TLSVerifyChainOnly {
			selectedTransport = "Verify Chain Only"
		} else if cluster.TLSSkipVerify {
			selectedTransport = "Skip Verify"
		} else if cluster.Insecure {
			selectedTransport = "No TLS"
		}
		if cluster.CertificateAuthorityData != "" {
			caData.Set(cluster.CertificateAuthorityData)
		}
	}
	transportSecurity := widget.NewRadioGroup([]string{
		"Verify Chain Only",
		"Skip Verify",
		"No TLS",
	}, func(s string) {
		switch s {
		case "Verify Chain Only":
			insecure.Set(false)
			verifyChainOnly.Set(true)
			skipVerify.Set(false)
		case "Skip Verify":
			insecure.Set(false)
			verifyChainOnly.Set(false)
			skipVerify.Set(true)
		case "No TLS":
			insecure.Set(true)
			verifyChainOnly.Set(false)
			skipVerify.Set(false)
		case "":
			insecure.Set(false)
			verifyChainOnly.Set(false)
			skipVerify.Set(false)
		}
	})
	transportSecurity.SetSelected(selectedTransport)
	transportSecurity.Required = false
	transportSecurity.Horizontal = true
	transportFormItem := widget.NewFormItem("Transport Security", transportSecurity)
	caEntry := widget.NewMultiLineEntry()
	caEntry.Bind(caData)
	caEntry.SetPlaceHolder("Base64 PEM encoded CA certificate")
	caEntry.ActionItem = app.newPEMFileSelector(caData)
	caEntry.Validator = validatePEMData(false)
	caFormItem := widget.NewFormItem("CA Certificate", caEntry)

	// Authentication
	authMethods := []string{"None", "Basic", "LDAP", "mTLS"}
	authConfigContainer := fyne.NewContainerWithLayout(layout.NewVBoxLayout())
	var currentAuthConfig fyne.CanvasObject
	authMethodSelect := widget.NewSelect(authMethods, func(s string) {
		cur, err := currentAuthMethod.Get()
		if err != nil && cur == s {
			return
		}
		currentAuthMethod.Set(s)
		switch s {
		case "Basic", "LDAP":
			form := newUserPassForm(username, password)
			if currentAuthConfig != nil {
				authConfigContainer.Remove(currentAuthConfig)
			}
			authConfigContainer.Add(form)
			currentAuthConfig = form
		case "mTLS":
			form := app.newMTLSForm(certData, keyData)
			if currentAuthConfig != nil {
				authConfigContainer.Remove(currentAuthConfig)
			}
			authConfigContainer.Add(form)
			currentAuthConfig = form
		case "None":
			authConfigContainer.RemoveAll()
		}
	})
	if !isNew {
		profile := currentConfig.GetContext(name)
		user := currentConfig.GetUser(profile.User)
		if user.BasicAuthUsername != "" && user.BasicAuthPassword != "" {
			username.Set(user.BasicAuthUsername)
			password.Set(user.BasicAuthPassword)
			authMethodSelect.SetSelected("Basic")
			currentAuthMethod.Set("Basic")
			return
		}
		if user.LDAPUsername != "" && user.LDAPPassword != "" {
			username.Set(user.LDAPUsername)
			password.Set(user.LDAPPassword)
			authMethodSelect.SetSelected("LDAP")
			currentAuthMethod.Set("LDAP")
			return
		}
		if user.ClientCertificateData != "" && user.ClientKeyData != "" {
			certData.Set(user.ClientCertificateData)
			keyData.Set(user.ClientKeyData)
			authMethodSelect.SetSelected("mTLS")
			currentAuthMethod.Set("mTLS")
			return
		}
		authMethodSelect.SetSelected("None")
	}
	authMethodFormItem := widget.NewFormItem("Authentication", authMethodSelect)

	// Submit handler
	onSubmit := func(ok bool) {
		if !ok {
			return
		}
		configPath := app.Preferences().StringWithFallback(preferenceConfigFile, config.DefaultConfigPath)
		name, _ := profileName.Get()
		server, _ := serverAddress.Get()
		insecure, _ := insecure.Get()
		verifyChainOnly, _ := verifyChainOnly.Get()
		skipVerify, _ := skipVerify.Get()
		caData, _ := caData.Get()
		authMethod, _ := currentAuthMethod.Get()
		if isNew {
			// Write the new profile to the config.
			clusterConfigName := fmt.Sprintf("%s-cluster", name)
			userConfigName := fmt.Sprintf("%s-user", name)
			currentConfig.Contexts = append(currentConfig.Contexts, config.Context{
				Name: name,
				Context: config.ContextConfig{
					Cluster: clusterConfigName,
					User:    userConfigName,
				},
			})
			currentConfig.Clusters = append(currentConfig.Clusters, config.Cluster{
				Name: clusterConfigName,
				Cluster: config.ClusterConfig{
					Server:                   server,
					Insecure:                 insecure,
					TLSVerifyChainOnly:       verifyChainOnly,
					TLSSkipVerify:            skipVerify,
					CertificateAuthorityData: caData,
				},
			})
			userConfig := config.User{
				Name: userConfigName,
				User: config.UserConfig{},
			}
			switch authMethod {
			case "Basic":
				user, _ := username.Get()
				pass, _ := password.Get()
				userConfig.User.BasicAuthUsername = user
				userConfig.User.BasicAuthPassword = pass
			case "LDAP":
				user, _ := username.Get()
				pass, _ := password.Get()
				userConfig.User.LDAPUsername = user
				userConfig.User.LDAPPassword = pass
			case "mTLS":
				cert, _ := certData.Get()
				key, _ := keyData.Get()
				userConfig.User.ClientCertificateData = cert
				userConfig.User.ClientKeyData = key
			}
			currentConfig.Users = append(currentConfig.Users, userConfig)
			// Save the config
			err := app.cli.SaveConfig(configPath)
			if err != nil {
				app.log.Error("error saving config", "error", err.Error())
				dialog.ShowError(fmt.Errorf("Error saving configuration: %w", err), app.main)
			}
			app.reloadProfileSelector()
			return
		}
		// Update the existing profile.
		profile := currentConfig.GetContext(name)
		cluster := currentConfig.GetCluster(profile.Cluster)
		user := currentConfig.GetUser(profile.User)
		cluster.Server = server
		cluster.Insecure = insecure
		cluster.TLSVerifyChainOnly = verifyChainOnly
		cluster.TLSSkipVerify = skipVerify
		cluster.CertificateAuthorityData = caData
		switch authMethod {
		case "Basic":
			user.BasicAuthUsername, _ = username.Get()
			user.BasicAuthPassword, _ = password.Get()
			user.LDAPUsername = ""
			user.LDAPPassword = ""
			user.ClientCertificateData = ""
			user.ClientKeyData = ""
		case "LDAP":
			user.LDAPUsername, _ = username.Get()
			user.LDAPPassword, _ = password.Get()
			user.BasicAuthUsername = ""
			user.BasicAuthPassword = ""
			user.ClientCertificateData = ""
			user.ClientKeyData = ""
		case "mTLS":
			user.ClientCertificateData, _ = certData.Get()
			user.ClientKeyData, _ = keyData.Get()
			user.BasicAuthUsername = ""
			user.BasicAuthPassword = ""
			user.LDAPUsername = ""
			user.LDAPPassword = ""
		case "None":
			user.BasicAuthUsername = ""
			user.BasicAuthPassword = ""
			user.LDAPUsername = ""
			user.LDAPPassword = ""
			user.ClientCertificateData = ""
			user.ClientKeyData = ""
		}
		// Save the config
		err := app.cli.SaveConfig(configPath)
		if err != nil {
			app.log.Error("error saving config", "error", err.Error())
			dialog.ShowError(fmt.Errorf("Error saving configuration: %w", err), app.main)
		}
		app.reloadProfileSelector()
	}

	dialog.ShowForm(title, "Save", "Cancel", []*widget.FormItem{
		nameFormItem,
		serverFormItem,
		transportFormItem,
		caFormItem,
		authMethodFormItem,
		widget.NewFormItem("", authConfigContainer),
	}, onSubmit, app.main)
}

func (app *App) profileOptions() []string {
	profiles := make([]string, 0)
	if config := app.cli.Config(); len(config.Contexts) > 0 {
		app.currentProfile.Set(config.CurrentContext)
		for _, ctx := range config.Contexts {
			profiles = append(profiles, ctx.Name)
		}
	} else {
		profiles = append(profiles, noProfiles)
	}
	return profiles
}

func (app *App) newMTLSForm(certData, keyData binding.String) fyne.CanvasObject {
	certEntry := widget.NewPasswordEntry()
	keyEntry := widget.NewPasswordEntry()
	certEntry.SetPlaceHolder("Base64 PEM encoded certificate")
	keyEntry.SetPlaceHolder("Base64 PEM encoded private key")
	certEntry.Bind(certData)
	keyEntry.Bind(keyData)
	certEntry.ActionItem = app.newPEMFileSelector(certData)
	keyEntry.ActionItem = app.newPEMFileSelector(keyData)
	certEntry.Validator = validatePEMData(true)
	keyEntry.Validator = validatePEMData(true)
	form := widget.NewForm(
		widget.NewFormItem("Certificate", certEntry),
		widget.NewFormItem("Private Key", keyEntry),
	)
	return form
}

func (app *App) newPEMFileSelector(target binding.String) fyne.CanvasObject {
	return widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				app.log.Error("error opening file", "error", err.Error())
				return
			}
			defer reader.Close()
			data, err := io.ReadAll(reader)
			if err != nil {
				app.log.Error("error reading file", "error", err.Error())
				return
			}
			block, _ := pem.Decode(data)
			if block == nil {
				fname := filepath.Base(reader.URI().String())
				dialog.ShowError(fmt.Errorf("%s is not a valid PEM-encoded file", fname), app.main)
				return
			}
			target.Set(base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(data))
		}, app.main)
	})
}

func newUserPassForm(username, password binding.String) fyne.CanvasObject {
	usernameEntry := widget.NewEntryWithData(username)
	usernameEntry.SetPlaceHolder("Enter username")
	usernameEntry.Wrapping = fyne.TextWrapOff
	usernameEntry.Validator = func(s string) error {
		if s == "" {
			return errors.New("username is required")
		}
		return nil
	}
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.Bind(password)
	passwordEntry.SetPlaceHolder("Enter password")
	passwordEntry.Wrapping = fyne.TextWrapOff
	passwordEntry.Validator = func(s string) error {
		if s == "" {
			return errors.New("password is required")
		}
		return nil
	}
	form := widget.NewForm(
		widget.NewFormItem("Username", usernameEntry),
		widget.NewFormItem("Password", passwordEntry),
	)
	return form
}

func validatePEMData(required bool) func(string) error {
	return func(val string) error {
		if val == "" {
			if required {
				return errors.New("data is required")
			}
			return nil
		}
		data, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return fmt.Errorf("invalid base64 data: %w", err)
		}
		block, _ := pem.Decode(data)
		if block == nil {
			return errors.New("invalid PEM data")
		}
		return nil
	}
}
