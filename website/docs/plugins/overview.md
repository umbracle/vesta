---
title: Overview
description: Cache every single thing your app could ever do ahead of time, so your code never even has to run at all.
---

**Vesta** is a plugin-based architecture that uses plugins written in [Starlark](https://bazel.build/rules/language) to describe how to deploy and manage a blockchain node.

In this section you will find all the available client deployments on **Vesta** and their configurations.

## Roadmap

- Decouple the catalog from the control plane. Currently, the catalog (`Starlark` files) are shipped inside the **Vesta** binary. In the future, this catalog will reside in one or several external repositories so that both Control plane and the Plugins can have different release cycles.

- Establish a framework to test plugins end-to-end to ensure their correctness.

- Expand the plugins outside the Ethereum ecosystem. If there is any client or network you have interest on please open a [Github issue](https://github.com/umbracle/vesta/issues).
