---
title: Teku
---

[Teku](https://docs.teku.consensys.net) is an Ethereum consensus client in Java.

## Chains

`Teku` is available for `mainnet`, `goerli` and `sepolia`.

## Parameters

- `execution_node` (string): Endpoint of the Ethereum execution node to use.
- `use_checkpoint` (bool: false): Whether to use checkpoint initial sync.
- `archive` (bool: false): Enables archival node mode.

## General Options:

Parameters that are common to all the clients as part of the [deploy](/docs/cli/deploy) command:

- `log_level`: (string: info): Verbosity level of the logs emitted by the client.
- `metrics`: (bool: true): Whether or not to enable [Prometheus](/docs/concepts/telemetry) metrics on the client.
