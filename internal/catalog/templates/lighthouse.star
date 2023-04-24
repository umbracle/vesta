version = "0.0.1"

chains = ["mainnet", "goerli", "sepolia"]

config = {
    "execution_node": {
        "type": "string",
        "required": True,
        "description": "Endpoint of the execution node",
    }
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


def generate(obj):
    t = {
        "image": "sigp/lighthouse",
        "tag": "v4.0.1",
        "args": [
            "lighthouse",
            "bn",
            "--network",
            obj["chain"],
            "--datadir",
            "/data",
            "--http",
            "--http-address",
            "0.0.0.0",
            "--http-port",
            "5052",
            "--execution-jwt",
            "/var/lib/jwtsecret/jwt.hex",
            "--execution-endpoint",
            "http://" + obj["execution_node"] + ":8551",
            "--metrics-address",
            "0.0.0.0",
            "--metrics-port",
            "8008",
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["metrics"]:
        t["args"].extend(["--metrics"])
        t["telemetry"] = {"port": 8008, "path": "metrics"}

    return {"node": t, "babel": babel}
