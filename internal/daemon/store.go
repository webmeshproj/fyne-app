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

	"github.com/webmeshproj/webmesh/pkg/cmd/ctlcmd/config"
	"github.com/webmeshproj/webmesh/pkg/mesh"
)

func newMeshConn(ctx context.Context, cfg *config.Config, opts ConnectOptions) (mesh.Mesh, error) {
	storeopts := newStoreOptions(cfg, opts)
	st, err := mesh.New(storeopts)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 30
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(opts.ConnectTimeout))
	defer cancel()
	err = st.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open mesh: %w", err)
	}
	return st, nil
}

func newStoreOptions(cfg *config.Config, opts ConnectOptions) *mesh.Options {
	storeOpts := mesh.NewOptions()
	storeOpts.Raft.InMemory = true
	storeOpts.Raft.ListenAddress = fmt.Sprintf(":%d", opts.RaftPort)
	storeOpts.Raft.LeaveOnShutdown = true
	storeOpts.Mesh.NoIPv4 = opts.NoIPv4
	storeOpts.Mesh.NoIPv6 = opts.NoIPv6
	storeOpts.Mesh.GRPCPort = int(opts.GRPCPort)
	storeOpts.WireGuard.InterfaceName = opts.InterfaceName
	storeOpts.WireGuard.ListenPort = int(opts.ListenPort)
	storeOpts.WireGuard.ForceTUN = opts.ForceTUN
	storeOpts.WireGuard.PersistentKeepAlive = time.Second * 10
	ctx := cfg.GetContext(opts.Profile)
	user := cfg.GetUser(ctx.User)
	if user.BasicAuthPassword != "" && user.BasicAuthUsername != "" {
		storeOpts.Mesh.NodeID = user.BasicAuthUsername
		storeOpts.Auth.Basic = &mesh.BasicAuthOptions{
			Username: user.BasicAuthUsername,
			Password: user.BasicAuthPassword,
		}
	}
	if user.LDAPPassword != "" && user.LDAPUsername != "" {
		storeOpts.Mesh.NodeID = user.LDAPUsername
		storeOpts.Auth.LDAP = &mesh.LDAPAuthOptions{
			Username: user.LDAPUsername,
			Password: user.LDAPPassword,
		}
	}
	if user.ClientKeyData != "" && user.ClientCertificateData != "" {
		storeOpts.Auth.MTLS = &mesh.MTLSOptions{
			CertData: user.ClientCertificateData,
			KeyData:  user.ClientKeyData,
		}
	}
	cluster := cfg.GetCluster(ctx.Cluster)
	storeOpts.Mesh.JoinAddress = cluster.Server
	if cluster.Insecure {
		storeOpts.TLS.Insecure = true
	} else {
		if cluster.CertificateAuthorityData != "" {
			storeOpts.TLS.CAData = cluster.CertificateAuthorityData
		}
		if cluster.TLSSkipVerify {
			storeOpts.TLS.InsecureSkipVerify = true
		}
		if cluster.TLSVerifyChainOnly {
			storeOpts.TLS.VerifyChainOnly = true
		}
	}
	return storeOpts
}
