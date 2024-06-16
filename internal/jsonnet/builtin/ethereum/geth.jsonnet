local task = import 'std/task.jsonnet';

task {
  name: 'geth',
  image: 'ethereum/client-go',
  chains: {
    mainnet: {
      max_version: 'v1.2.3',
      min_version: 'v1.2.0',
    },
    sepolia: {
      max_version: 'v1.2.3',
      min_version: 'v1.2.0',
    },
  },
  ports: [],
  volumes: [
    {
      name: 'data',
      path: '/data',
      properties: {
        dbengine: {
          type: 'string',
        },
      },
    },
  ],
  generate: function(input) {
    args: [
      '--datadir',
      '/data',
      '--http.addr',
      '0.0.0.0',
      '--http',
      '--http.port',
      '8545',
      '--http.vhosts',
      '*',
      '--http.corsdomain',
      '*',
      '--authrpc.addr',
      '0.0.0.0',
      '--authrpc.port',
      '8551',
      '--authrpc.vhosts',
      '*',
      '--authrpc.jwtsecret',
      '/var/lib/jwtsecret/jwt.hex',
      '--metrics.addr',
      '0.0.0.0',
      '--ipcdisable',
      '--maxpeers',
      '32',
    ] + (
      if input.chain == 'sepolia' then [
        '--sepolia',
      ] else if input.chain == 'holesky' then [
        '--holesky',
      ] else []
    ),
    files: {
      '/var/lib/jwtsecret/jwt.hex': '04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf',
    },
  },
}
