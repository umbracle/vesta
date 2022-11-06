# Vesta

Vesta is a modular deployment platform for blockchains. Blockchain deployments are described as [Cue](https://cuelang.org) scripts that define inputs/outputs and tasks to be deployed (as Docker containers). The control plane automatically handles the full lifecycle of the deployments (create, update, destroy) and unifies under a single interface common tasks like exporting observability, tracking sync state or disk storage management.

Unlike other available options ([DappNode](https://www.dappnode.io/), [Sedge](https://github.com/NethermindEth/sedge) or [eth-docker](https://github.com/eth-educators/eth-docker)), Vesta is not only a template engine on top of docker/docker-compose, but a complete ad-hoc control-plane fully integrated with the lifecycle of a blockchain node.

## Usage

Deploy the Vesta control plane and the local runner:

```
$ go run cmd/main.go server
```

Deploy a `Geth` execution node for `goerli`:

```
$ go run cmd/main.go deploy --type Geth --chain goerli
```

Deploy a `Teku` beacon node for `goerli`:

```
$ go run cmd/main.go deploy --type Teku --chain goerli
```

## Catalog

The list of all the built-in deployments available can be found [here](https://github.com/umbracle/vesta/blob/main/pkg/vesta.io/vesta/schema.cue).

In the future, the `catalog` will be available on a remote repository.
