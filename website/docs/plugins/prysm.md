---
title: Prysm
---

[Prysm](https://prysmaticlabs.com/) is an Ethereum consensus client in Go.

## Chains

`Prysm` is available for `mainnet`.

## Parameters

- `execution_node` (string): Endpoint of the Ethereum execution node to use.
- `use_checkpoint` (bool: false): Whether to use checkpoint initial sync.
- `archive` (bool: false): Enables archival node mode.

## General Options:

Parameters that are common to all the clients as part of the [deploy](/docs/cli/deploy) command:

- `log_level`: (string: info): Verbosity level of the logs emitted by the client.
- `metrics`: (bool: true): Whether or not to enable [Prometheus](/docs/concepts/telemetry) metrics on the client.
