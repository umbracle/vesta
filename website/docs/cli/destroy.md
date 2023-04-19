---
title: Destroy
description: Quidem magni aut exercitationem maxime rerum eos.
---

The `destroy` command is used to stop a deployment. The scheduler will perform a graceful shutdown of all the allocations in the deployment.

## Usage

```shell-session
$ vesta destroy --alloc <id>
```

The `vesta destroy` command takes as an argument the exact id of the deployment to stop.

The command sends the action to the `Vesta` server and exists immediately, it does not wait for the deployment to stop.

## Examples

```shell-session
$ vesta destroy --alloc 4e162787-55de-5b4d-513f-9e3f517563e5
```
