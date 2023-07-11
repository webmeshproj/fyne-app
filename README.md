# Webmesh Client Application

This is a GUI client application for the webmesh project.
It is written in Go using the [fyne](https://fyne.io/) toolkit.
It can be used to connect to a webmesh for client-only use.
In the future it may be extended to include support for server functionality.

# Development

## Prerequisites

- An accessible webmesh server. You can use the `docker-compose` in this repository to run one locally.
- Go 1.20 or later

## Building and Running

To build the application, run `go build` in the root of this repository or you can use `go run main.go` to build and run it in one step.
Since the application needs to manage network interfaces and routes, a privleged daemon is required if the app is not run as root.
The daemon can be started by running

```sh
sudo go run main.go --daemon
```

The application itself takes an optional `--config` file to preload connection settings.
The configuration included in this repository is for the local docker-compose setup.
To use it, run

```sh
go run main.go --config config.yaml
```
