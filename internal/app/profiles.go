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

func (app *App) reloadProfileSelector() {
	contexts := make([]string, 0)
	defaultContext := ""
	if config := app.cli.Config(); config != nil {
		app.currentProfile.Set(config.CurrentContext)
		defaultContext = config.CurrentContext
		for _, ctx := range config.Contexts {
			contexts = append(contexts, ctx.Name)
		}
	}
	contexts = append(contexts, "New Context")
	app.profiles.Options = contexts
	app.profiles.Selected = defaultContext
	app.profiles.OnChanged = func(selected string) {
		if selected == "New Context" {
			// TODO: New profile dialog.
			currentProfile, err := app.currentProfile.Get()
			if err == nil {
				app.profiles.SetSelected(currentProfile)
			}
			return
		}
		// TODO: If already connected to a profile, prompt if okay to switch connections
		// and reconnect.
		app.currentProfile.Set(selected)
	}
}
