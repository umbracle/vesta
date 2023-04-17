version = "0.0.1"


def chains():
    return ["mainnet", "goerli", "sepolia"]


def config():
    return {
        "dbengine": {
            "type": "string",
            "description": "Database engine to use (leveldb, pebble)",
            "allowed_values": ["leveldb", "pebble"],
            "force_new": True,
            "default": "leveldb",
        }
    }


def generate(obj):
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
        ],
        "data": {"/var/lib/jwtsecret/jwt.hex": ""},
        "volumes": {"data": {"path": "/data"}},
    }

    if obj["chain"] != "mainnet":
        t["args"].extend(["--" + obj["chain"]])

    if obj["metrics"]:
        t["args"].extend(["--metrics"])
        t["telemetry"] = {"port": 6060, "path": "debug/metrics/prometheus"}

    if obj["dbengine"] == "pebble":
        t["args"].extend(["--db.engine", "pebble"])

    return {"node": t}
