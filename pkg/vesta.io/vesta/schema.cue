package vesta

#Mount: {
	dest: string
	type: "string"
	contents: string
}

#Port: {
	bind: string
	port: number

	type: string
	{
		type: "metrics"
	}
}

#Volume: {
	path: string
}

#Telemetry: {
	port: number
	path: string
}

#Runtime: {
	mounts: [name=string]: #Mount
	volumes: [name=string]: #Volume
	ports: [name=string]:  #Port

	image: string
	tag:   string | *"latest"

	env: [name=string]: string
	args: [...string]

	telemetry?: #Telemetry
}

// Description of a blockchain node
#Node: {
	@obj("node")
	
	// input of the object
	input: {
		chain: string
		metrics: bool | *true
	}

	// list of execution containers
	tasks: [name=string]: #Runtime

	// output of the object
	output: {}
}

_jwt_token: "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"

Geth: {
	#Node

	input: {
		max_peers: string | *"23"
	}

	tasks: {
		node: #Runtime & {
			image: "ethereum/client-go"
			tag:   "v1.10.21"

			ports: {
				"http": {
					port: 8545
				}
				"engine": {
					port: 8551
				}
				"metrics": {
					port: 8585
					type: "metrics"
				}
			}

			volumes: {
				data: {
					path: "/data"
				}
			}

			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				"--datadir", "/data",
				
				if input.chain == "mainnet" {
					"--mainnet"
				},
				if input.chain == "goerli" {
					"--goerli"
				},
				
				// Http api
				"--http.addr", "0.0.0.0",
				"--http", "--http.port", "8545",
				"--http.vhosts", "*",
				"--http.corsdomain", "*",

				// Engine api
				"--authrpc.addr", "0.0.0.0",
				"--authrpc.port", "8551",
				"--authrpc.vhosts", "*",
				"--authrpc.jwtsecret", "/var/lib/jwtsecret/jwt.hex",

				// Metrics
				"--metrics.addr", "0.0.0.0",
				if input.metrics {
					"--metrics",
				}
			]

			if input.metrics {
				telemetry: {
					port: 6060
					path: "debug/metrics/prometheus"
				}
			}
		}
	}
}

Nethermind: {
	#Node

	input: {}

	tasks: {
		node: #Runtime & {
			image: "nethermind/nethermind"
			tag:   "1.14.6"

			volumes: {
				data: {
					path: "/data"
				}
			}
			
			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				"--datadir",
				"/data",

				"--config",
				if input.chain == "goerli" {
					"goerli",
				},

				"--JsonRpc.Enabled", "true",
				"--JsonRpc.Host", "0.0.0.0",
				"--JsonRpc.Port", "8545",
				"--JsonRpc.EngineHost", "0.0.0.0",
				"--JsonRpc.EnginePort", "8551",
				"--JsonRpc.JwtSecretFile", "/var/lib/jwtsecret/jwt.hex",

				"--Metrics.ExposePort", "6060",
				if input.metrics {
					"--Metrics.Enabled",
				}
				if input.metrics {
					"true",
				}
			]

			if input.metrics {
				telemetry: {
					port: 6060
					path: "metrics"
				}
			}
		}
	}
}

Besu: {
	#Node

	input: {}

	tasks: {
		node: #Runtime & {
			image: "hyperledger/besu"
			tag:   "latest"

			volumes: {
				data: {
					path: "/data"
				}
			}

			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				"--data-path",
				"/data",

				"--network",
				if input.chain == "goerli" {
					"goerli",
				}

				"--rpc-http-enabled",
				"--rpc-http-host", "0.0.0.0",
				"--rpc-http-port", "8545",
				"--rpc-http-cors-origins", "*",

				"--host-allowlist", "*",
				"--engine-host-allowlist", "*",
				"--engine-jwt-secret", "/var/lib/jwtsecret/jwt.hex",
				"--engine-rpc-port", "8551",

				if input.metrics {
					"--metrics-enabled",
				}
				"--metrics-host", "0.0.0.0",
				"--metrics-port", "6060",
			]

			if input.metrics {
				telemetry: {
					port: 6060
					path: "metrics"
				}
			}
		}
	}
}

Teku: {
	#Node

	input: {
		execution_node: string
	}

	tasks: {
		node: #Runtime & {
			image: "consensys/teku"
			tag:   "22.8.0"

			volumes: {
				data: {
					path: "/data"
				}
			}

			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				"--network",
				if input.chain == "goerli" {
					"goerli"
				},

				"--data-base-path", "/data",

				"--ee-endpoint",
				"http://"+input.execution_node+":8551",
				"--ee-jwt-secret-file",
				"/var/lib/jwtsecret/jwt.hex",

				// metrics
				"--metrics-host-allowlist", "*",
				"--metrics-port", "8008",
				"--metrics-interface", "0.0.0.0",
				if input.metrics {
					"--metrics-enabled"
				}
			]

			if input.metrics {
				telemetry: {
					port: 8008
					path: "metrics"
				}
			}
		}
	}
}

Lighthouse: {
	#Node

	input: {
		execution_node: string
	}

	tasks: {
		node: #Runtime & {
			image: "sigp/lighthouse"
			tag:   "v3.3.0"

			volumes: {
				data: {
					path: "/data"
				}
			}

			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				"lighthouse",
				"bn",

				"--network",
				if input.chain == "goerli" {
					"goerli"
				},

				"--datadir", "/data",

				"--http",
				"--http-address", "0.0.0.0",
				"--http-port", "5052",

				"--execution-jwt", "/var/lib/jwtsecret/jwt.hex",
				"--execution-endpoint", "http://"+input.execution_node+":8551",

				if input.metrics {
					"--metrics"
				}
    			"--metrics-address", "0.0.0.0",
				"--metrics-port", "8008",
			]

			if input.metrics {
				telemetry: {
					port: 8008
					path: "metrics"
				}
			}
		}
	}
}

Prysm: {
	#Node

	input: {
		execution_node: string
	}

	tasks: {
		node: #Runtime & {
			image: "gcr.io/prysmaticlabs/prysm/beacon-chain"
			tag: "v3.1.2"

			volumes: {
				data: {
					path: "/data"
				}
			}

			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				if input.chain == "goerli" {
					"--goerli"
				}

				"--datadir", "/data",

				"--execution-endpoint", "http://"+input.execution_node+":8551",
      			"--jwt-secret", "/var/lib/jwtsecret/jwt.hex",

      			"--grpc-gateway-host", "0.0.0.0",
      			"--grpc-gateway-port", "5052",

	        	"--accept-terms-of-use",

      			"--monitoring-host", "0.0.0.0",
      			"--monitoring-port", "8008",
			]

			if input.metrics {
				telemetry: {
					port: 8008
					path: "metrics"
				}
			}
		}
	}
}
