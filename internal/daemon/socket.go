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

import (
	"context"
	"net"
	"runtime"
)

// listen returns a new listener for the daemon socket.
func listen() (net.Listener, error) {
	return net.Listen("unix", getSocketPath())
}

// dial returns a new connection to the daemon socket. It matches the signature
// of net.DialContext so it can be used as a dialer.
func dial(ctx context.Context, _, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", getSocketPath())
}

// getSocketPath returns the path to the socket file for communicating
// with the helper daemon.
func getSocketPath() string {
	if runtime.GOOS == "windows" {
		return "\\\\.\\pipe\\webmesh.sock"
	}
	return "/var/run/webmesh.sock"
}
