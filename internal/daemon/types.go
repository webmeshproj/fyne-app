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

import "errors"

// IsNotConnected returns true if the error signals not being connected
// to the mesh.
func IsNotConnected(err error) bool {
	return err == errNotConnected || err.Error() == errNotConnected.Error()
}

// ConnectOptions are the options for connecting to the mesh.
type ConnectOptions struct {
	// Profile is the profile to use for connecting.
	Profile string `json:"profile"`
	// InterfaceName is the name to set for the wireguard interface.
	InterfaceName string `json:"interfaceName"`
	// ForceTUN is whether to force the use of a TUN interface.
	ForceTUN bool `json:"forceTUN"`
	// ListenPort is the port for wireguard to listen on.
	ListenPort uint16 `json:"listenPort"`
	// RaftPort is the port to use for the Raft transport.
	RaftPort uint16 `json:"raftPort"`
	// GRPCPort is the port to use for the gRPC transport.
	GRPCPort uint16 `json:"grpcPort"`
	// NoIPv4 is whether to not use IPv4 when joining the cluster.
	NoIPv4 bool `json:"noIPv4"`
	// NoIPv6 is whether to not use IPv6 when joining the cluster.
	NoIPv6 bool `json:"noIPv6"`
	// LocalDNS is whether to start a local MeshDNS server.
	LocalDNS bool `json:"localDNS"`
	// LocalDNSPort is the port to use for the local MeshDNS server.
	LocalDNSPort uint16 `json:"localDNSPort"`
	// ConnectTimeout is tjhe timeout to use for connecting in seconds.
	// If 0, a default timeout of 30 seconds is used.
	ConnectTimeout int `json:"connectTimeout"`
}

// ErrNotConnected is returned when the daemon is not connected to a mesh.
var errNotConnected = errors.New("not connected")

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
