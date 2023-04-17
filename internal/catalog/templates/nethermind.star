version = "0.0.1"


def chains():
    return ["mainnet", "goerli", "sepolia"]


def config():
    return {}


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
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["metrics"]:
        t["args"].extend(["--Metrics.Enabled", "true"])
        t["telemetry"] = {"port": 6060, "path": "metrics"}

    return {"node": t}
