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
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
)

var socketPath = getSocketPath()

// listen returns a new listener for the daemon socket.
func listen() (net.Listener, error) {
	if runtime.GOOS != "windows" {
		// Ensure the socket directory exists.
		sockDir := filepath.Dir(socketPath)
		if err := os.MkdirAll(sockDir, 0750); err != nil {
			return nil, err
		}
		// Ensure the socket directory has the correct permissions.
		if err := os.Chmod(sockDir, 0750); err != nil {
			return nil, fmt.Errorf("chmod unix socket directory: %w", err)
		}
		// Change the group ownership to the webmesh group if it exists.
		group, err := user.LookupGroup("webmesh")
		if err == nil {
			gid, err := strconv.Atoi(group.Gid)
			if err != nil {
				return nil, fmt.Errorf("invalid gid: %w", err)
			}
			err = os.Chown(sockDir, -1, gid)
			if err != nil {
				return nil, fmt.Errorf("chown unix socket directory: %w", err)
			}
		}
		// Remove any existing socket file.
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return net.Listen("unix", socketPath)
}

// dial returns a new connection to the daemon socket. It matches the signature
// of net.DialContext so it can be used as a dialer.
func dial(ctx context.Context, _, _ string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
}

// getSocketPath returns the path to the socket file for communicating
// with the helper daemon.
func getSocketPath() string {
	if runtime.GOOS == "windows" {
		return "\\\\.\\pipe\\webmesh.sock"
	}
	return "/var/run/webmesh/webmesh.sock"
}
