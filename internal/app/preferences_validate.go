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
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2/data/binding"
)

func validatePreferences() error {
	for _, val := range []func() error{
		validatePorts,
		validateConnectTimeout,
	} {
		if err := val(); err != nil {
			return err
		}
	}
	return nil
}

func validatePorts() error {
	for _, bd := range []struct {
		name string
		val  binding.String
	}{
		{"WireGuard port", wireguardPort},
		{"Raft port", raftPort},
		{"gRPC port", grpcPort},
	} {
		val, err := bd.val.Get()
		if err != nil {
			return err
		}
		if val == "" {
			return fmt.Errorf("%s is required", bd.name)
		}
		if _, err := strconv.ParseUint(val, 10, 16); err != nil {
			return fmt.Errorf("%s is not a valid port: %s", bd.name, val)
		}
	}
	return nil
}

func validateConnectTimeout() error {
	val, err := connectTimeout.Get()
	if err != nil {
		return err
	}
	_, err = time.ParseDuration(val)
	if err != nil {
		return fmt.Errorf("connect timeout is invalid: %w", err)
	}
	return nil
}
