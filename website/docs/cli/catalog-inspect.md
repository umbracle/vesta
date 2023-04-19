---
title: Catalog list
---

The `catalog inspect` command is used to display the information of a specific plugin in the catalog.

## Usage

```shell-session
$ vesta catalog inspect [options] <name>
```

The `catalog inspect` command takes as an argument the name of the plugin to inspect.

## Examples

```shell-session
$ vesta catalog inspect prysm
Name = prysm

Input fields
Name            Type    Required  Description
execution_node  string  true      Endpoint of the execution node
use_checkpoint  bool    false     Whether to use checkpoint initial sync

Available chains
Name
mainnet
goerli
sepolia
```
