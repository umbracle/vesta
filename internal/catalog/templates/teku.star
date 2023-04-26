version = "0.0.1"

name = "teku"

chains = ["mainnet", "goerli", "sepolia"]

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
    "all": "ALL",
    "debug": "DEBUG",
    "info": "INFO",
    "warn": "WARN",
    "error": "ERROR",
    "silent": "OFF",
}


def generate(obj):
    t = {
        "image": "consensys/teku",
        "tag": "23.3.0",
        "args": [
            "--data-base-path",
            "/data",
            "--ee-endpoint",
            "http://" + obj["execution_node"] + ":8551",
            "--ee-jwt-secret-file",
            "/var/lib/jwtsecret/jwt.hex",
            "--metrics-host-allowlist",
            "*",
            "--metrics-port",
            "8008",
            "--metrics-interface",
            "0.0.0.0",
            "--rest-api-enabled",
            "--rest-api-host-allowlist",
            "*",
            "--rest-api-interface",
            "0.0.0.0",
            "--rest-api-port",
            "5052",
            "--log-destination",
            "CONSOLE",
            "--logging",
            verbosity_levels[obj["log_level"]],
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["use_checkpoint"]:
        t["args"].extend(
            [
                "--initial-state",
                getBeaconCheckpoint(obj["chain"])
                + "/eth/v2/debug/beacon/states/finalized",
            ]
        )

    if obj["archive"]:
        t["args"].extend(["--data-storage-mode", "prune"])
    else:
        t["args"].extend(["--data-storage-mode", "archive"])

    if obj["chain"] != "mainnet":
        t["args"].extend(["--network", obj["chain"]])

    if obj["metrics"]:
        t["args"].extend(["--metrics-enabled"])
        t["telemetry"] = {"port": 8008, "path": "metrics"}

    return {"node": t, "babel": babel}


def getBeaconCheckpoint(chain):
    if chain == "mainnet":
        return "https://beaconstate.info"
    elif chain == "goerli":
        return "https://goerli.beaconstate.info"
    elif chain == "sepolia":
        return "https://sepolia.beaconstate.info"
