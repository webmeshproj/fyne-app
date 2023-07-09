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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"sync"

	v1 "github.com/webmeshproj/api/v1"
	"github.com/webmeshproj/node/pkg/ctlcmd/config"
	"github.com/webmeshproj/node/pkg/store"
)

// Client is the client for the daemon.
type Client interface {
	// LoadConfig loads the client configuration from the given path.
	LoadConfig(path string) error
	// SaveConfig saves the client configuration to the given path.
	SaveConfig(path string) error
	// Config returns the current client configuration, or nil if none
	// is loaded.
	Config() *config.Config
	// Connect connects to the mesh.
	Connect(ctx context.Context, opts ConnectOptions) error
	// Disconnect disconnects from the mesh.
	Disconnect(ctx context.Context) error
	// InterfaceMetrics returns the metrics for the mesh interface.
	InterfaceMetrics(ctx context.Context) (*v1.InterfaceMetrics, error)
}

// ConnectOptions are the options for connecting to the mesh.
type ConnectOptions struct {
	// Profile is the profile to use for connecting.
	Profile string `json:"profile"`
	// ConnectTimeout is tjhe timeout to use for connecting in seconds.
	// If 0, a default timeout of 30 seconds is used.
	ConnectTimeout int `json:"connectTimeout"`
	// InterfaceName is the name to set for the wireguard interface.
	InterfaceName string `json:"interfaceName"`
	// ListenPort is the port for wireguard to listen on.
	ListenPort uint16 `json:"listenPort"`
	// ForceTUN is whether to force the use of a TUN interface.
	ForceTUN bool `json:"forceTUN"`
	// RaftPort is the port to use for the Raft transport.
	RaftPort uint16 `json:"raftPort"`
	// GRPCPort is the port to use for the GRPC transport.
	GRPCPort uint16 `json:"grpcPort"`
	// NoIPv4 is whether to not use IPv4 when joining the cluster.
	NoIPv4 bool `json:"noIPv4"`
	// NoIPv6 is whether to not use IPv6 when joining the cluster.
	NoIPv6 bool `json:"noIPv6"`
	// LocalDNS is whether to start a local MeshDNS server.
	LocalDNS bool `json:"localDNS"`
	// LocalDNSPort is the port to use for the local MeshDNS server.
	LocalDNSPort uint16 `json:"localDNSPort"`
}

type client struct {
	*http.Client
	configPath string
	config     *config.Config
	noDaemon   bool
	// Only valid when noDaemon is true.
	store store.Store
	mu    sync.Mutex
}

// NewClient returns a new client.
func NewClient() Client {
	return &client{
		// If we are root, we don't need to use the unix socket
		// if it does not exist.
		noDaemon: func() bool {
			_, err := os.Stat(getSocketPath())
			if runtime.GOOS == "windows" {
				user, err := user.Current()
				if err == nil {
					return user.Name == "SYSTEM" && os.IsNotExist(err)
				}
				return false
			}
			return os.Getuid() == 0 && os.IsNotExist(err)
		}(),
		Client: &http.Client{
			Transport: &http.Transport{
				DialContext: dial,
			},
		},
	}
}

func (c *client) LoadConfig(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	c.configPath = path
	c.config, err = config.FromFile(path)
	return err
}

func (c *client) SaveConfig(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.config == nil {
		return nil
	}
	return c.config.WriteTo(path)
}

func (c *client) Config() *config.Config {
	return c.config
}

func (c *client) Connect(ctx context.Context, opts ConnectOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noDaemon {
		var err error
		if c.store != nil {
			err = c.store.Close()
			if err != nil {
				return fmt.Errorf("close existing store: %w", err)
			}
		}
		c.store, err = newStore(ctx, c.config, opts)
		if err != nil {
			return err
		}
		return nil
	}
	req := &connectRequest{
		ConfigFile: c.configPath,
		Options:    opts,
	}
	return c.do(ctx, http.MethodPost, "/connect", req, nil)
}

func (c *client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noDaemon {
		if c.store == nil {
			return ErrNotConnected
		}
		err := c.store.Close()
		if err != nil {
			return fmt.Errorf("close store: %w", err)
		}
		c.store = nil
	}
	return c.do(ctx, http.MethodPost, "/disconnect", nil, nil)
}

func (c *client) InterfaceMetrics(ctx context.Context) (*v1.InterfaceMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noDaemon {
		if c.store == nil {
			return nil, ErrNotConnected
		}
		return c.store.WireGuard().Metrics()
	}
	var out v1.InterfaceMetrics
	return &out, c.do(ctx, http.MethodGet, "/interface-metrics", nil, &out)
}

func (c *client) do(ctx context.Context, method, path string, req, resp interface{}) error {
	var body io.ReadCloser = http.NoBody
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = io.NopCloser(bytes.NewReader(b))
	}
	r, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	res, err := c.Do(r)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var err daemonError
		if err := json.NewDecoder(res.Body).Decode(&err); err != nil {
			return fmt.Errorf("bad status: %s, decode error: %w", res.Status, err)
		}
		return &err
	}
	if resp != nil {
		if err := json.NewDecoder(res.Body).Decode(resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
