---
title: Scheduler
description: Quidem magni aut exercitationem maxime rerum eos.
---

**Vesta** runs blockchain nodes as [Docker](https://www.docker.com/) containers, which provides a deterministic execution environment and allows for easy management of different versions. Additionally, many client teams already ship their applications as Docker containers, making them straightforward to integrate.

In the Plugins section, it was explained that each deployment can define one or more tasks to run on the node. Typically, the main client is the primary task, with other sidecar applications for tracking sync state, for example. All the tasks in the same deployment run under the same network space.

Vesta ships with its own container scheduler and it does not rely on other external systems to do so (i.e. docker-compose). This makes Vesta lightweight and simple to run.
