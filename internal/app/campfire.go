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
	"io"
	"path"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
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

func (app *App) listRooms() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	c, err := app.dialNode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to dial node: %w", err)
	}
	defer c.Close()
	resp, err := v1.NewAppDaemonClient(c).Query(ctx, &v1.QueryRequest{
		Command: v1.QueryRequest_LIST,
		Query:   RoomsPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query rooms: %w", err)
	}
	defer resp.CloseSend()
	result, err := resp.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive query result: %w", err)
	}
	rooms := make([]string, 0, 10)
	for _, r := range result.GetValue() {
		r = strings.TrimPrefix(r, RoomsPrefix+"/")
		parts := strings.Split(r, "/")
		if len(parts) != 1 {
			continue
		}
		rooms = append(rooms, parts[0])
	}
	return rooms, nil
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
	roomName.Wrapping = fyne.TextWrapOff
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
	selfDestruct.Wrapping = fyne.TextWrapOff
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
		ourID, _ := app.nodeID.Get()
		// Add ourself as a member
		err = app.doPublish(ctx, &v1.PublishRequest{
			Key: MembersPath(roomName) + "/" + ourID,
			Ttl: durationpb.New(ttl),
		})
		if err != nil {
			app.log.Error("error adding member", "error", err.Error())
			dialog.ShowError(err, app.main)
			return
		}
		app.joinRooms = append(app.joinRooms, roomName)
	}, app.main)
}

func (app *App) onRoomSelected(index int) {
	if app.chatContainer.Hidden {
		return
	}
	app.chatGrid.Show()
	ctx := context.Background()
	ctx, app.cancelRoomSubscription = context.WithCancel(ctx)
	roomName, err := app.roomsList.GetItem(index)
	if err != nil {
		app.log.Error("error getting room name", "error", err.Error())
		return
	}
	roomNameValue, _ := roomName.(binding.String).Get()
	app.selectedRoom = roomNameValue
	c, err := app.dialNode(ctx)
	if err != nil {
		app.log.Error("error dialing node", "error", err.Error())
		return
	}
	// Check if we have already joined
	if !slices.Contains(app.joinRooms, roomNameValue) {
		// Join the room
		ourID, _ := app.nodeID.Get()
		err = app.doPublish(ctx, &v1.PublishRequest{
			Key: MembersPath(roomNameValue) + "/" + ourID,
		})
		if err != nil {
			app.log.Error("error joining room", "error", err.Error())
			return
		}
	}
	// List the current members
	cli := v1.NewAppDaemonClient(c)
	resp, err := cli.Query(ctx, &v1.QueryRequest{
		Command: v1.QueryRequest_LIST,
		Query:   MembersPath(roomNameValue),
	})
	if err != nil {
		app.log.Error("error listing members", "error", err.Error())
		return
	}
	defer resp.CloseSend()
	result, err := resp.Recv()
	if err != nil {
		app.log.Error("error receiving members", "error", err.Error())
		return
	}
	members := make([]string, 0, 10)
	for _, m := range result.GetValue() {
		m = strings.TrimPrefix(m, MembersPath(roomNameValue)+"/")
		parts := strings.Split(m, "/")
		if len(parts) != 1 {
			continue
		}
		members = append(members, parts[0])
	}
	// Write a header to the chat text grid
	app.chatText.SetText(fmt.Sprintf("Room: %s\nMembers: %s\n", roomNameValue, strings.Join(members, ", ")))
	stream, err := cli.Subscribe(ctx, &v1.SubscribeRequest{
		Prefix: RoomPath(roomNameValue),
	})
	if err != nil {
		app.log.Error("error subscribing to room", "error", err.Error())
		return
	}
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				app.log.Error("error receiving message", "error", err.Error())
				return
			}
			prefix := strings.TrimPrefix(msg.GetKey(), RoomPath(roomNameValue)+"/")
			parts := strings.Split(prefix, "/")
			switch parts[0] {
			case "members":
				if len(parts) != 2 {
					continue
				}
				// Emit a message to the chat text grid
				app.chatText.SetText(fmt.Sprintf("%sMember %s joined the room\n", app.chatText.Text(), parts[1]))
			case "messages":
				if len(parts) != 3 {
					continue
				}
				// Emit a message to the chat text grid
				from := parts[2]
				ts := parts[1]
				t, _ := time.Parse(time.RFC3339Nano, ts)
				tstr := t.Format(time.RFC3339)
				msg := strings.TrimSpace(msg.GetValue())
				app.chatText.SetText(fmt.Sprintf("%s%s [%s]: %s\n", app.chatText.Text(), from, tstr, msg))
			}
		}
	}()
}

func (app *App) onSendMessage(s string) {
	if s == "" {
		return
	}
	nodeID, _ := app.nodeID.Get()
	key := NewMessageKey(app.selectedRoom, nodeID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err := app.doPublish(ctx, &v1.PublishRequest{
		Key:   key,
		Value: s,
	})
	if err != nil {
		app.log.Error("error sending message", "error", err.Error())
		return
	}
	app.chatInput.SetText("")
}

func (app *App) onRoomUnselected(index int) {
	if app.chatContainer.Hidden {
		return
	}
	app.chatGrid.Hide()
	app.cancelRoomSubscription()
	app.chatText.SetText("")
}
