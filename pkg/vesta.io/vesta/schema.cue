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

#Runtime: {
	mounts: [name=string]: #Mount
	ports: [name=string]:  #Port

	image: string
	tag:   string | *"latest"

	env: [name=string]: string
	args: [...string]
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

			mounts: {
				"jwt": {
					dest:     "/var/lib/jwtsecret/jwt.hex"
					type:     "string"
					contents: _jwt_token
				}
			}

			args: [
				if input.chain == "mainnet" {
					"--mainnet"
				},
				if input.chain == "goerli" {
					"--goerli"
				},
				"--http", "--http.port", "8545",

				if input.metrics {
					"--metrics"
				}
			]
		}
	}
}

Teku: {
	#Node

	input: {}

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
				"http://127.0.0.1:8551",
				"--ee-jwt-secret-file",
				"/var/lib/jwtsecret/jwt.hex",
			]
		}
	}
}
