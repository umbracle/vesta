---
title: Announcing Vesta
description: Announcing Vesta, a unified control plane to deploy and manage blockchain infrastructure at scale.
slug: announcing-vesta
---

We are pleased to announce the official release of Vesta, a new open-source project that enables operators and infrastructure engineers to easily manage the deployment of blockchain clients.

Vesta is designed to provide a unified and consistent workflow to deploy and manage blockchain infrastructure at scale.

Traditionally, setting up a blockchain client has posed some challenges, including configuring networking best practices, navigating complex documentation guides, and keeping up with updates and forks. This has made it harder for users and companies to join decentralized networks and run their own infra.

Vesta provides a single and lightweight entry point that simplifies the configuration, setup, and management of any blockchain client. Out of the box, `Vesta` handles deployment, monitoring, logs management, metric collection, and notifications. It uses a plugin architecture to integrate and support different blockchain clients, networks, and infrastructures.

Check the Getting Started guide and download `Vesta` if you want to start using it right away.

## Addressing Complexity

Deploying and managing a production blockchain node is complex.

Though many teams provide now simple `docker-compose` templates to easily set up and join a decentralized network, this only solves a small subset of the problem.

As users digress from these default deployments, it becomes increasingly difficult to follow best practices, track client updates, or follow behavior and flag changes. Often, this becomes also a burden on the development team which has to provide support to answer these questions.

The challenges do not stop after the initial deployment is done, After the initial rollout, day two operations become critical to continuously monitor and maintain the node over time. This might involve setting up a telemetry stack to gather logs and metrics, monitor the sync state, follow version updates on Discord, and so on.

With `Vesta`, we aim to integrate all of these operations under the same stack. With one click, you can parametrize and deploy a production-ready blockchain client. It handles natively all the management and maintenance operations.

## Features

These are some of the features that `Vesta` provides out of the box:

- Lightweight: It does not require any external service (i.e. `docker-compose`) to run the clients locally. It ships with its own built-in scheduler.
- Node tracking: `Vesta` tracks natively the synchronization status of the nodes.
- Integrated metrics: All the Prometheus metrics emitted by the clients are automatically collected by `Vesta`. Then, they are labeled and exposed in a single entry point.

## An example workflow

This is an example to showcase how easy is to use Vesta to deploy a pair of Geth and Prysm nodes to join the Ethereum mainnet:

```go
$ vesta deploy --chain mainnet --type Geth --alias geth
$ vesta deploy --chain mainnet --type Prysm execution_node=geth use_checkpoint=true
```

## Extensible via plugins

**`Vesta`** is built on a plugin-based architecture, enabling developers to easily extend **`Vesta`** with new nodes and deployments.

**`Vesta Plugins`** are written in [Starlark](https://bazel.build/rules/language), a small and simple interpreted language with a Python-like syntax. Each plugin exposes the implementation of a single blockchain client (i.e. Geth, Prysm).

It ships with support for the major clients of the `Ethereum` network (both beacon and execution node). Over time, we hope to extend this catalog with more clients, networks, and different types of infrastructures.

If you are part of any blockchain client development team and want to add support for your node, feel free to fill out this contact form.

## Learn more

- Follow the [Getting Started](/docs/getting-started) guide to download `Vesta` and deploy an Ethereum mainnet stack.
- [Join the Discord](https://discord.gg/YajpNSvT22) channel to talk with the developers.
- [Check the catalog](/docs/plugins/overview) to see the complete list of available nodes to deploy.
