---
title: Besu
---

[Besu](https://www.hyperledger.org/use/besu) is an Apache 2.0 licensed, MainNet compatible, Ethereum execution client written in Java.

## Chains

`Besu` is available for `mainnet`, `goerli` and `sepolia`.

## Parameters

- `archive` (bool: false): Enables archival node mode.

## General Options:

Parameters that are common to all the clients as part of the [deploy](/docs/cli/deploy) command:

- `log_level`: (string: info): Verbosity level of the logs emitted by the client.
- `metrics`: (bool: true): Whether or not to enable [Prometheus](/docs/concepts/telemetry) metrics on the client.
