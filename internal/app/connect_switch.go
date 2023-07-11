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
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

const (
	switchDisconnected = 0
	switchConnecting   = 0.5
	switchConnected    = 1
)

type connectSwitch struct {
	widget.Slider
}

func newConnectSwitch() (*connectSwitch, binding.Float) {
	slider := &connectSwitch{}
	slider.ExtendBaseWidget(slider)
	connected := binding.NewFloat()
	slider.Bind(connected)
	slider.Min = 0
	slider.Max = 1
	slider.Step = 0.5
	slider.Orientation = widget.Horizontal
	return slider, connected
}

func (t *connectSwitch) Tapped(_ *fyne.PointEvent) {
	switch t.Value {
	case switchDisconnected:
		t.SetValue(switchConnecting)
	case switchConnected:
		t.SetValue(switchDisconnected)
	case switchConnecting:
		// Trigger the listener to cancel the connection.
		t.SetValue(switchDisconnected)
	}
}
