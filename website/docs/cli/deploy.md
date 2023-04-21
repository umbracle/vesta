---
title: Deploy
---

The `deploy` command is used to submit new deployments to **Vesta** or to update existing ones.

## Usage

```shell-session
$ vesta deploy [params] [options]
```

## Options

- `type`: (string): The name of the plugin to use.
- `chain`: (string): The chain you want to deploy. The plugin will filter if the chain name is correct.
- `alloc`: (string): The allocation if we are performing an update.
- `metrics`: (bool: true): Whether the node tracks metrics or not.
- `alias`: (string): The alias of the node. If set, you can use this name instead of the deployment id to refer to this node.
- `log-level`: (string): Logging level for the output log of the nodes. Available options: (`all`, `debug`, `info`, `warn`, `error`, `silent`). It defaults to `info`.

Each plugin also defines custom parameters that can be queried with the `catalog inspect` command. Those specific fields are passed as values to the cli but without a flag, for example `param=val` instead of `--param=val`.

## Examples

Deploy a Geth node:

```shell-session
$ vesta deploy --type Geth --chain goerli
c4809d78-aae8-d2bc-f886-31fb65fb97ce
```

Deploy a Prysm beacon node:

```shell-session
$ vesta deploy --type prysm --chain mainnet execution_node=c4809d78-aae8-d2bc-f886-31fb65fb97ce use_checkpoint=true
4e162787-55de-5b4d-513f-9e3f517563e5
```

Update the node to disable metrics:

```shell-session
$ vesta deploy --type Geth --chain goerli --metrics=false --alloc c4809d78-aae8-d2bc-f886-31fb65fb97ce
```
