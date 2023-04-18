version = "0.0.1"

chains = ["mainnet", "goerli", "sepolia"]

config = {}


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
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["metrics"]:
        t["args"].extend(["--metrics-enabled"])
        t["telemetry"] = {"port": 6060, "path": "metrics"}

    return {"node": t}
