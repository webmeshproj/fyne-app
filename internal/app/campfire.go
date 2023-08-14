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
	"path"
	"strings"
	"time"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	v1 "github.com/webmeshproj/api/v1"
	"github.com/webmeshproj/webmesh/pkg/campfire"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	// CampFirePrefix is the storage prefix for the campfire chat.
	CampFirePrefix = "/campfire-chat"
	// RoomsPrefix is the prefix for campfire chat rooms.
	RoomsPrefix = CampFirePrefix + "/rooms"
)

// RoomPath returns the storage path for a room.
func RoomPath(roomName string) string {
	return path.Join(RoomsPrefix, roomName)
}

// MembersPath returns the storage path for a room's members.
func MembersPath(roomName string) string {
	return path.Join(RoomPath(roomName), "members")
}

// MessagesPath returns the storage path for a room's messages.
func MessagesPath(roomName string) string {
	return path.Join(RoomPath(roomName), "messages")
}

// NewMessageKey returns a new message key for publishing to a room.
func NewMessageKey(roomName string, from string) string {
	t := time.Now().UTC().Format(time.RFC3339Nano)
	return path.Join(MessagesPath(roomName), t, from)
}

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
	app.campfireURL.Set(uri.EncodeURI())
}

func (app *App) onNewChatRoom() {
	if app.chatContainer.Hidden {
		return
	}
	roomName := widget.NewEntry()
	roomName.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("room name cannot be empty")
		}
		current, _ := app.roomsList.Get()
		for _, r := range current {
			if r == s {
				return errors.New("room already exists")
			}
		}
		return nil
	}
	selfDestruct := widget.NewEntry()
	selfDestruct.SetText("1h")
	selfDestruct.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		_, err := time.ParseDuration(s)
		return err
	}
	dialog.ShowForm("New Chat Room", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Room Name", roomName),
		widget.NewFormItem("Self Destruct", selfDestruct),
	}, func(ok bool) {
		if !ok {
			return
		}
		roomName := strings.TrimSpace(roomName.Text)
		var ttl time.Duration
		if strings.TrimSpace(selfDestruct.Text) != "" {
			ttl, _ = time.ParseDuration(selfDestruct.Text)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := app.doPublish(ctx, &v1.PublishRequest{
			Key: RoomPath(roomName),
			Ttl: durationpb.New(ttl),
		})
		if err != nil {
			app.log.Error("error creating room", "error", err.Error())
			dialog.ShowError(err, app.main)
			return
		}
		err = app.roomsList.Append(roomName)
		if err != nil {
			app.log.Error("error appending room", "error", err.Error())
			dialog.ShowError(err, app.main)
			return
		}
	}, app.main)
}

func (app *App) onRoomSelected(index int) {
	if app.chatContainer.Hidden {
		return
	}
}

func (app *App) onRoomUnselected(index int) {
	if app.chatContainer.Hidden {
		return
	}
}
