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

package daemon

import "fmt"

// ErrNotConnected is returned when the daemon is not connected to a mesh.
var ErrNotConnected = fmt.Errorf("not connected")

// daemonError is an error returned by the daemon.
type daemonError struct {
	Message string `json:"message"`
}

// Error returns the error message.
func (e daemonError) Error() string {
	return e.Message
}

// daemonOKResponse is a generic response sent by the daemon.
type daemonOKResponse struct {
	OK bool `json:"ok"`
}

// connectRequest is the request sent to the daemon to connect to a mesh.
type connectRequest struct {
	ConfigFile string         `json:"configFile"`
	Options    ConnectOptions `json:"options"`
}
