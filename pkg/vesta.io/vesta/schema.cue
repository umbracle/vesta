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

#Sync: {
	port: number
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
	sync?: #Sync
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

		babel: #Runtime & {
			image: "babel",
			tag: "dev",
			
			args: [
				"--plugin", "ethereum_el", "server", "url=http://${ALLOCID}:8545"
			]

			sync: {
				port: 2020
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
