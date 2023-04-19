---
title: Getting started
description: Cache every single thing your app could ever do ahead of time, so your code never even has to run at all.
---

In this quick example we are going to run a set of Ethereum nodes for mainnet (without a validator) with `Prysm` and `Geth`.

Download Vesta:

```shell-session
$ curl https://releases.umbracle.xyz/vesta/v0.1.1/vesta_v0.1.1_linux_amd64.zip > vesta_v0.1.1_linux_amd64.zip
$ unzip vesta_v0.1.1_linux_amd64.zip
Archive:  vesta_v0.1.1_linux_amd64.zip
  inflating: vesta
```

Deploy the Vesta control plane and the local runner:

```shell-session
$ vesta server
```

Now, deploy the `Geth` node:

```shell-session
$ vesta deploy --type Geth --chain mainnet
ead45a46-fcf5-faf6-bd35-39dcff42aaaf
```

The command outputs the id of the Geth node.

Use `deployment status` command to check the status of the deployment:

```shell-session
$ vesta deployment status ead45a46
ID       = ead45a46-fcf5-faf6-bd35-39dcff42aaaf
Status   = Running
Sequence = 0

Tasks
ID    Name                Image    State
node  ethereum/client-go  v1.11.5  Running
```

Deploy a `Prysm` node connected with the `Geth` node that will use checkpoints for fast syncing:

```shell-session
$ vesta deploy --type prysm --chain mainnet execution_node=ead45a46-fcf5-faf6-bd35-39dcff42aaaf use_checkpoint=true
88756ce3-fa97-6d14-de73-8b0b858ef9b2
```

Now, both nodes are up and running:

```shell-session
$ vesta deployment list
Name                                  Status
88756ce3-fa97-6d14-de73-8b0b858ef9b2  Running
ead45a46-fcf5-faf6-bd35-39dcff42aaaf  Running
```
