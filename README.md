# Webmesh Client Application

This is a GUI client application for the webmesh project.
It is written in Go using the [fyne](https://fyne.io/) toolkit.
It can be used to connect to a webmesh for client-only use.
In the future it may be extended to include support for server functionality.

# Development

## Prerequisites

- An accessible webmesh node. You can use the `docker-compose` in this repository to run one locally.
- Go 1.20 or later

## Building and Running

To build the application, run `go build` in the root of this repository or you can use `go run main.go` to build and run it in one step.
Since the application needs to manage network interfaces and routes, a privileged daemon is required if the app is not run as root.
The daemon can be started by running a `webmesh-node` with the `--app-daemon` flag.
You can also use the `docker-compose` in this repository to run a node with the daemon enabled.

```sh
webmesh-turn --app-daemon --app-daemon-bind tcp://127.0.0.1:8080
```

By default, on unix-like systems, the daemon will listen on a unix socket at `/var/run/webmesh/webmesh.sock`.
The permissions of the socket will be set to 770 with an ownership of `root:root` or `root:webmesh` if the group exists.
This will be the preferred method of communication for deployment targets.
