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
	"sync/atomic"

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
	// Connected returns true if the client is connected to the mesh.
	Connected() bool
	// Connect connects to the mesh.
	Connect(ctx context.Context, opts ConnectOptions) error
	// Disconnect disconnects from the mesh.
	Disconnect(ctx context.Context) error
	// InterfaceMetrics returns the metrics for the mesh interface.
	InterfaceMetrics(ctx context.Context) (*v1.InterfaceMetrics, error)
}

type client struct {
	cli        *http.Client
	configPath string
	config     *config.Config
	connected  atomic.Bool
	noDaemon   bool
	mu         sync.Mutex
	// Only valid when noDaemon is true.
	store store.Store
}

// NewClient returns a new client.
func NewClient() Client {
	return &client{
		// If we are root, we don't need to use the unix socket
		// if it does not exist.
		noDaemon: func() bool {
			_, err := os.Stat(socketPath)
			if runtime.GOOS == "windows" {
				user, err := user.Current()
				if err == nil {
					return user.Name == "SYSTEM" && os.IsNotExist(err)
				}
				return false
			}
			return os.Getuid() == 0 && os.IsNotExist(err)
		}(),
		cli: &http.Client{
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

func (c *client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected.Load()
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
			c.connected.Store(false)
		}
		c.store, err = newStore(ctx, c.config, opts)
		if err != nil {
			return err
		}
		c.connected.Store(true)
		return nil
	}
	req := &connectRequest{
		ConfigFile: c.configPath,
		Options:    opts,
	}
	err := c.do(ctx, http.MethodPost, "/connect", req, nil)
	if err == nil {
		c.connected.Store(true)
	}
	return err
}

func (c *client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noDaemon {
		if c.store == nil {
			return errNotConnected
		}
		err := c.store.Close()
		if err != nil {
			return fmt.Errorf("close store: %w", err)
		}
		c.store = nil
		c.connected.Store(false)
	}
	err := c.do(ctx, http.MethodPost, "/disconnect", nil, nil)
	if err == nil {
		c.connected.Store(false)
	}
	return err
}

func (c *client) InterfaceMetrics(ctx context.Context) (*v1.InterfaceMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.noDaemon {
		if c.store == nil {
			return nil, errNotConnected
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
	res, err := c.cli.Do(r)
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
	if resp == nil {
		// Make sure we read the body so the connection can be reused.
		resp = &daemonOKResponse{}
	}
	if err := json.NewDecoder(res.Body).Decode(resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
