# Vesta

[![Chat Badge]][chat link]

[chat badge]: https://img.shields.io/badge/chat-discord-%237289da
[chat link]: https://discord.gg/YajpNSvT22

Vesta is a modular deployment platform for blockchains. Blockchain deployments are described as [Cue](https://cuelang.org) scripts that define inputs/outputs and tasks to be deployed (as Docker containers). The control plane automatically handles the full lifecycle of the deployments (create, update, destroy) and unifies under a single interface common tasks like exporting [observability](https://github.com/umbracle/vesta#Telemetry), tracking sync state or disk storage management.

Unlike other available options ([DappNode](https://www.dappnode.io/), [Sedge](https://github.com/NethermindEth/sedge) or [eth-docker](https://github.com/eth-educators/eth-docker)), Vesta is not only a template engine on top of docker/docker-compose, but a complete ad-hoc control-plane fully integrated with the lifecycle of a blockchain node.

## Usage

Deploy the Vesta control plane and the local runner:

```
$ go run cmd/main.go server [--volume /data]
```

You can optionally set a volume directory (`--volume`) as the location to store persistent data.

Deploy a `Geth` execution node for `goerli`:

```
$ go run cmd/main.go deploy --type Geth --chain goerli
c03b3642-4732-2794-8a53-57cf1972bdde
```

Deploy a `Teku` beacon node for `goerli` connected to the `Geth` node from the previous step:

```
$ go run cmd/main.go deploy --type Teku --chain goerli execution_node=c03b3642-4732-2794-8a53-57cf1972bdde
```

Update a node:

```
$ go run cmd/main.go deploy --type Geth --chain goerli metrics=false --alloc c03b3642-4732-2794-8a53-57cf1972bdde
```

Destroy a node:

```
$ go run cmd/main.go destroy --alloc c03b3642-4732-2794-8a53-57cf1972bdde
```

## Catalog

The list of all the built-in deployments available can be found [here](https://github.com/umbracle/vesta/blob/main/pkg/vesta.io/vesta/schema.cue).

In the future, the `catalog` will be available on a remote repository.

## Telemetry

The Vesta telemetry system aggregates the logs emitted by the blockchain nodes and exposes them on a single Prometheus endpoint at `localhost:5555`.

Then, you can process these logs with `Grafana`:

```
cd telemetry
docker-compose up
```

Open the browner at `localhost:3000` and use `admin` as username and password. There is a pre-configured datasource with all the aggregated Vesta node metrics.

In the future, Vesta will emit these metrics to external log platforms like AWS Cloudtrail or Datadog.
