local input = std.native('ctx')();

{
  artifacts: [] + (
    if input.chain != 'mainnet' then [
      {
        dst: '/data/genesis.ssz',
        src: 'https://github.com/eth-clients/merge-testnets/raw/main/' + input.chain + '/genesis.ssz',
      },
    ]
  ),
  args: [
    '--datadir',
    '/data',
    '--grpc-gateway-host',
    '0.0.0.0',
    '--grpc-gateway-port',
    '5052',
    '--accept-terms-of-use',
    '--monitoring-host',
    '0.0.0.0',
    '--monitoring-port',
    '8008',
  ] + (
    if input.chain != 'mainnet' then [
      '--' + input.chain,
      '--genesis-state',
      '/data/genesis.ssz',
    ]
  ) + (
    if input.archive then [
      '--slots-per-archive-point',
      '32',
    ] else []
  ),
  files: {
    '/var/lib/jwtsecret/jwt.hex': '04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf',
  },
}
