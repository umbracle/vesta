---
title: Geth
---

[Geth](https://geth.ethereum.org/) (go-ethereum) is a Go implementation of an Ethereum execution client.

## Chains

`Geth` is available for `mainnet`, `goerli` and `sepolia`.

## Parameters

- `dbengine`: Database engine to use (leveldb, pebble). It defaults to `leveldb`.
- `max_peers` (number: 50): Maximum number of network peers.
- `archive` (bool: false): Enables archival node mode.

## General Options:

Parameters that are common to all the clients as part of the [deploy](/docs/cli/deploy) command:

- `log_level`: (string: info): Verbosity level of the logs emitted by the client.
- `metrics`: (bool: true): Whether or not to enable [Prometheus](/docs/concepts/telemetry) metrics on the client.
