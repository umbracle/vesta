{
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
  tags: {},
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
}
