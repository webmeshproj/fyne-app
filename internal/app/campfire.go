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
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"github.com/webmeshproj/webmesh/pkg/campfire"
)

var campfireURL = binding.NewString()

func (app *App) onNewCampfire() {
	psk, err := campfire.GeneratePSK()
	if err != nil {
		// This should never happen
		dialog.ShowError(err, app.main)
		return
	}
	turnServersPref := app.Preferences().StringWithFallback(preferenceTURNServers, "")
	if strings.TrimSpace(turnServersPref) == "" {
		dialog.ShowError(errors.New("no TURN servers configured, add them in the preferences"), app.main)
		return
	}
	campTurnServers := strings.Split(strings.TrimSpace(turnServersPref), ",")
	for i, server := range campTurnServers {
		campTurnServers[i] = strings.TrimPrefix(server, "turn:")
	}
	uri := &campfire.CampfireURI{
		PSK:         psk,
		TURNServers: campTurnServers,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err = app.startCampfire(ctx, uri)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to start campfire: %w", err), app.main)
		return
	}
	campfireURL.Set(uri.EncodeURI())
}
