---
title: Plugins
---

**Vesta** is a tool to deploy and manage blockchain nodes at scale. **Vesta** is built on a plugin-based architecture, enabling developers to easily extend **Vesta** with new nodes and deployments.

Logically, **Vesta** is split in two components: **Control plane** and **Plugins**. The Control plane leverages the Plugins to deploy and manage blockchain nodes.

## Control plane

The Control plane is a statically compiled binary written in Golang. It is the main entrypoint for users of **Vesta**. Its main reponsabilities are:

- Management of the persistent state.
- Execution of the local Docker scheduler.
- Run the API service (GRPC).

## Vesta Plugins

**Vesta Plugins** are written in [Starlark](https://bazel.build/rules/language), a small and simple interpreted language with a Python-like syntax. Each plugin exposes the implementation of a single blockchain client (i.e. Geth, Prysm). The primary responsabilities of a plugin are:

- Define the chains in which the client can run.
- Define the input parameters for the client (i.e. max number of peers).
- Declare how to translate the input parameters into a `Deployment` object. The `Deployment` defines the set of `Tasks` to run as part of the client. Each `Task` represents an executable `Docker` container. The `Task` also define some extra information (i.e. Prometheus endpoint) that help the `Control plane` manage all the blockchain nodes in an integrated way.

You can find the list of available plugins and their parameters in the [`Plugins`](/plugins) section.

As of now, the full catalog of Plugins is released under the same binary as the Control plane. In the future, these two components will be separated entities and the Control plane will be able to fetch the catalog from external sources.
