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
	"time"

	"github.com/webmeshproj/node/pkg/ctlcmd/config"
	"github.com/webmeshproj/node/pkg/store"
	"golang.org/x/exp/slog"
)

func newStore(ctx context.Context, cfg *config.Config, opts ConnectOptions) (store.Store, error) {
	storeopts := newStoreOptions(cfg, opts)
	st, err := store.New(storeopts)
	if err != nil {
		return nil, fmt.Errorf("new store: %w", err)
	}
	err = st.Open()
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(opts.ConnectTimeout))
	defer cancel()
	<-st.ReadyNotify(ctx)
	if ctx.Err() != nil {
		err = st.Close()
		if err != nil {
			slog.Default().Error("error closing store", "error", err.Error())
		}
		return nil, fmt.Errorf("wait for store ready: %w", ctx.Err())
	}
	return st, nil
}

func newStoreOptions(cfg *config.Config, opts ConnectOptions) *store.Options {
	storeOpts := store.NewOptions()
	storeOpts.Raft.InMemory = true
	storeOpts.Raft.ListenAddress = fmt.Sprintf(":%d", opts.RaftPort)
	storeOpts.Raft.LeaveOnShutdown = true
	storeOpts.Raft.ShutdownTimeout = time.Second * 10
	storeOpts.Mesh.NoIPv4 = opts.NoIPv4
	storeOpts.Mesh.NoIPv6 = opts.NoIPv6
	storeOpts.Mesh.GRPCPort = int(opts.GRPCPort)
	storeOpts.WireGuard.InterfaceName = opts.InterfaceName
	storeOpts.WireGuard.ListenPort = int(opts.ListenPort)
	storeOpts.WireGuard.ForceTUN = opts.ForceTUN
	storeOpts.WireGuard.PersistentKeepAlive = time.Second * 10
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 30
	}
	storeOpts.Mesh.JoinTimeout = time.Second * time.Duration(opts.ConnectTimeout)
	// TODO: Add auth config from profile.
	return storeOpts
}
