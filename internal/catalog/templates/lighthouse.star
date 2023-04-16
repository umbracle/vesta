version = "0.0.1"

def chains():
    return ["mainnet", "goerli", "sepolia"]

def config():
    return {
        "execution_node": {
            "required": True,
            "type": "string",
            "description": ""
        }
    }

def generate(obj):
    t = {
        "image": "sigp/lighthouse",
        "tag": "v4.0.1",
        "args": [
            "b",
            "network",
            obj["chain"]
        ]
    }

    t["args"].extend(["a", "1"])

    return t
