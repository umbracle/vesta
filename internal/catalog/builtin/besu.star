version = "0.0.1"

name = "besu"

chains = ["mainnet", "goerli", "sepolia"]

config = {
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
        "ethereum_el",
        "server",
        "url=http://0.0.0.0:8545",
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
        "image": "hyperledger/besu",
        "tag": "latest",
        "args": [
            "--data-path",
            "/data",
            "--network",
            obj["chain"],
            "--rpc-http-enabled",
            "--rpc-http-host",
            "0.0.0.0",
            "--rpc-http-port",
            "8545",
            "--rpc-http-cors-origins",
            "*",
            "--host-allowlist",
            "*",
            "--engine-host-allowlist",
            "*",
            "--engine-jwt-secret",
            "/var/lib/jwtsecret/jwt.hex",
            "--engine-rpc-port",
            "8551",
            "--metrics-host",
            "0.0.0.0",
            "--metrics-port",
            "6060",
            "--logging",
            verbosity_levels[obj["log_level"]],
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["archive"]:
        t["args"].extend(["--data-storage-format", "FOREST", "--sync-mode", "FULL"])
    else:
        t["args"].extend(["--data-storage-format", "BONSAI", "--sync-mode", "X_SNAP"])

    if obj["metrics"]:
        t["args"].extend(["--metrics-enabled"])
        t["telemetry"] = {"port": 6060, "path": "metrics"}

    return {"node": t, "babel": babel}
