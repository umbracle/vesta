version = "0.0.1"

name = "prysm"

chains = ["mainnet"]

config = {
    "execution_node": {
        "type": "string",
        "required": True,
        "description": "Endpoint of the execution node",
    },
    "use_checkpoint": {
        "type": "bool",
        "description": "Whether to use checkpoint initial sync",
    },
    "archive": {
        "type": "bool",
        "description": "Enables archival node mode",
        "default": False,
    },
}

babel = {
    "image": "ghcr.io/umbracle/babel",
    "tag": "v0.0.1",
    "args": [
        "--plugin",
        "ethereum_cl",
        "server",
        "url=http://0.0.0.0:5052",
    ],
}

verbosity_levels = {
    "all": "debug",
    "debug": "debug",
    "info": "info",
    "warn": "warn",
    "error": "error",
    "silent": "error",
}


def generate(obj):
    t = {
        "image": "gcr.io/prysmaticlabs/prysm/beacon-chain",
        "tag": "v4.0.0",
        "args": [
            "--datadir",
            "/data",
            "--execution-endpoint",
            "http://" + obj["execution_node"] + ":8551",
            "--jwt-secret",
            "/var/lib/jwtsecret/jwt.hex",
            "--grpc-gateway-host",
            "0.0.0.0",
            "--grpc-gateway-port",
            "5052",
            "--accept-terms-of-use",
            "--monitoring-host",
            "0.0.0.0",
            "--monitoring-port",
            "8008",
            "--verbosity",
            verbosity_levels[obj["log_level"]],
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["archive"]:
        # https://docs.prylabs.network/docs/advanced/beacon_node_api
        t["args"].extend(["--slots-per-archive-point", "32"])

    if obj["use_checkpoint"]:
        url = getBeaconCheckpoint(obj["chain"])

        t["args"].extend(
            ["--checkpoint-sync-url", url, "--genesis-beacon-api-url", url]
        )

    if obj["chain"] != "mainnet":
        # It requires to download artifacts (TODO)
        # add '--sepolia' or '--goerli' (it defaults to mainnet)
        t["args"].extend(["--" + obj["chain"]])

    if obj["metrics"]:
        t["telemetry"] = {"port": 8008, "path": "metrics"}

    return {"node": t, "babel": babel}


def getBeaconCheckpoint(chain):
    if chain == "mainnet":
        return "https://beaconstate.info"
    elif chain == "goerli":
        return "https://goerli.beaconstate.info"
    elif chain == "sepolia":
        return "https://sepolia.beaconstate.info"
