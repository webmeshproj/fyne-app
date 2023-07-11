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

package main

import (
	"flag"
	"path/filepath"

	"github.com/webmeshproj/app/internal/app"
	"github.com/webmeshproj/app/internal/daemon"
)

func main() {
	configFile := flag.String("config", "", "Path to a configuration file to preload")
	helperDaemon := flag.Bool("daemon", false, "Run the helper daemon")
	daemonInsecure := flag.Bool("insecure", false, "Run the helper daemon in insecure mode")
	flag.Parse()
	// TODO: set up logging
	// Should tee to a file in the user's home directory when running the app
	if *helperDaemon {
		daemon.Run(*daemonInsecure)
		return
	}
	config := *configFile
	if config != "" {
		var err error
		config, err = filepath.Abs(config)
		if err != nil {
			panic(err)
		}
	}
	app.New(config).Run()
}
