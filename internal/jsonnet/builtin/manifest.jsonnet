local geth = import 'geth.jsonnet';
local prysm = import 'prysm.jsonnet';

{
  network: 'Ethereum',
  chains: {
    mainnet: {
      type: 'production',
    },
    sepolia: {
      type: 'testnet',
    },
    holesky: {
      type: 'testnet',
    },
  },
  nodes: [
    geth,
    prysm,
  ],
}
