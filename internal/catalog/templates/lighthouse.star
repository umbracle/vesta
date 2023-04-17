version = "0.0.1"


def chains():
    return ["mainnet", "goerli", "sepolia"]


def config():
    return {
        "execution_node": {
            "type": "string",
            "required": True,
            "description": "Endpoint of the execution node",
        }
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
        "data": {"/var/lib/jwtsecret/jwt.hex": ""},
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["metrics"]:
        t["args"].extend(["--metrics"])
        t["telemetry"] = {"port": 8008, "path": "metrics"}

    return {"node": t}
