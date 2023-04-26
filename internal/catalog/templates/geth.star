version = "0.0.1"

name = "geth"

chains = ["mainnet", "goerli", "sepolia"]

config = {
    "dbengine": {
        "type": "string",
        "description": "Database engine to use (leveldb, pebble)",
        "allowed_values": ["leveldb", "pebble"],
        "force_new": True,
        "default": "leveldb",
    },
    "max_peers": {
        "type": "int",
        "description": "Maximum number of network peers",
        "default": 50,
    },
    "archive": {
        "type": "bool",
        "description": "Enables archival node mode",
        "default": False,
    },
}

verbosity_levels = {
    "all": "5",
    "debug": "4",
    "info": "3",
    "warn": "2",
    "error": "1",
    "silent": "0",
}

babel = {
    "image": "ghcr.io/umbracle/babel",
    "tag": "v0.0.1",
    "args": [
        "--plugin",
        "ethereum_el",
        "server",
        "url=http://0.0.0.0:8545",
    ],
}


def generate(obj):
    verbosity = verbosity_levels[obj["log_level"]]

    t = {
        "image": "ethereum/client-go",
        "tag": "v1.11.5",
        "args": [
            "--datadir",
            "/data",
            "--http.addr",
            "0.0.0.0",
            "--http",
            "--http.port",
            "8545",
            "--http.vhosts",
            "*",
            "--http.corsdomain",
            "*",
            "--authrpc.addr",
            "0.0.0.0",
            "--authrpc.port",
            "8551",
            "--authrpc.vhosts",
            "*",
            "--authrpc.jwtsecret",
            "/var/lib/jwtsecret/jwt.hex",
            "--metrics.addr",
            "0.0.0.0",
            "--verbosity",
            verbosity,
            "--maxpeers",
            str(obj["max_peers"]),
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["archive"]:
        t["args"].extend(["--syncmode", "full", "--gcmode", "archive"])
    else:
        t["args"].extend(["--syncmode", "snap"])

    if obj["chain"] != "mainnet":
        t["args"].extend(["--" + obj["chain"]])

    if obj["metrics"]:
        t["args"].extend(["--metrics"])
        t["telemetry"] = {"port": 6060, "path": "debug/metrics/prometheus"}

    if obj["dbengine"] == "pebble":
        t["args"].extend(["--db.engine", "pebble"])

    return {"node": t, "babel": babel}
