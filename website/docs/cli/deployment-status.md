---
title: Deployment status
description: Quidem magni aut exercitationem maxime rerum eos.
---

The `deployment status` command is used to display the status of a deployment.

## Usage

```shell-session
$ vesta deployment status <id>
```

The `deployment status` command takes as an argument the id (or a prefix) of the deployment to display.

## Examples

```shell-session
$ vesta deployment status 4e162
ID       = 4e162787-55de-5b4d-513f-9e3f517563e5
Status   = Running
Sequence = 0

Tasks
ID    Name                                     Image   State
node  gcr.io/prysmaticlabs/prysm/beacon-chain  v4.0.0  Running
```
