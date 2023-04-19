---
title: Server
description: Quidem magni aut exercitationem maxime rerum eos.
---

The `server` command starts the **Vesta** server process and the local scheduler agent.

## Usage

```shell-session
$ vesta server [options]
```

## Options

- `volume`: The path of the place to store the persistent data.

## Examples

Run the `Vesta` server with an external mounted volume

```shell-session
$ vesta server --volume /mnt/external
2023-04-18T14:08:08.625+0200 [INFO]  vesta: GRPC Server started: addr=localhost:4003
2023-04-18T14:08:08.625+0200 [WARN]  vesta: no volume is set
2023-04-18T14:08:08.625+0200 [INFO]  vesta.agent: agent started
2023-04-18T14:08:08.633+0200 [INFO]  vesta.agent: Prometheus server started: addr==127.0.0.1:5555
```
