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
    "image": "babel",
    "tag": "dev",
    "args": [
        "--plugin",
        "ethereum_cl",
        "server",
        "url=http://0.0.0.0:5052",
    ],
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
        ],
        "data": {
            "/var/lib/jwtsecret/jwt.hex": "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"
        },
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["chain"] != "mainnet":
        t["args"].extend(["--network", "--" + obj["chain"]])

    if obj["metrics"]:
        t["args"].extend(["--metrics-enabled"])
        t["telemetry"] = {"port": 8008, "path": "metrics"}

    return {"node": t, "babel": babel}
