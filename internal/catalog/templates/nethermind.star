version = "0.0.1"

name = "nethermind"

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
    "all": "DEBUG",
    "debug": "DEBUG",
    "info": "INFO",
    "warn": "WARN",
    "error": "ERROR",
    "silent": "ERROR",
}


def generate(obj):
    t = {
        "image": "nethermind/nethermind",
        "tag": "1.17.3",
        "args": [
            "--datadir",
            "/data",
            "--config",
            obj["chain"],
            "--JsonRpc.Enabled",
            "true",
            "--JsonRpc.Host",
            "0.0.0.0",
            "--JsonRpc.Port",
            "8545",
            "--JsonRpc.EngineHost",
            "0.0.0.0",
            "--JsonRpc.EnginePort",
            "8551",
            "--JsonRpc.JwtSecretFile",
            "/var/lib/jwtsecret/jwt.hex",
            "--Metrics.ExposePort",
            "6060",
            "--log",
            verbosity_levels[obj["log_level"]],
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["archive"]:
        t["args"].extend(
            [
                "--Sync.DownloadBodiesInFastSync",
                "false",
                "--Sync.DownloadReceiptsInFastSync",
                "false",
                "--Sync.FastSync",
                "false",
                "--Sync.SnapSync",
                "false",
                "--Sync.FastBlocks",
                "false",
                "--Pruning.Mode",
                "None",
                "--Sync.PivotNumber",
                "0",
            ]
        )
    else:
        t["args"].extend(["--Sync.SnapSync", "true"])

    if obj["metrics"]:
        t["args"].extend(["--Metrics.Enabled", "true"])
        t["telemetry"] = {"port": 6060, "path": "metrics"}

    return {"node": t, "babel": babel}
