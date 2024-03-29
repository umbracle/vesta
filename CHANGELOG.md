# 0.1.2 (Unreleased)

BUG FIXES:

- catalog: `Sepolia` and `Goerli` on `Prysm` [[GH-77](https://github.com/umbracle/eth2-validator/issues/77)]
- scheduler: Handle destroy desired status updates [[GH-65](https://github.com/umbracle/eth2-validator/issues/65)]

IMPROVEMENTS:

- core: Hook to download artifacts [[GH-77](https://github.com/umbracle/eth2-validator/issues/77)]
- catalog: Add `--catalog` flag to server to load an external catalog [[GH-75](https://github.com/umbracle/eth2-validator/issues/75)]
- core: Add sync tracker with babel [[GH-41](https://github.com/umbracle/eth2-validator/issues/41)]
- catalog: Add logging field in catalog [[GH-62](https://github.com/umbracle/eth2-validator/issues/62)]
- core: Add an alias for the deployment [[GH-23](https://github.com/umbracle/eth2-validator/issues/23)]

# 0.1.1 (18 April, 2023)

- feat: Use Id or Prefix in `deployment status` [[GH-22](https://github.com/umbracle/eth2-validator/issues/22)]
- feat: List available chains in the plugin [[GH-48](https://github.com/umbracle/eth2-validator/issues/48)]
- feat: Add state persistence [[GH-8](https://github.com/umbracle/eth2-validator/issues/8)]
- feat: Add cli `catalog list` and `catalog inspect` commands [[GH-6](https://github.com/umbracle/eth2-validator/issues/6)]
- feat: Add cue scripts for `Nethermind` and `Besu` [[GH-32](https://github.com/umbracle/eth2-validator/issues/32)]
- feat: Link dependent deployments by DNS [[GH-7](https://github.com/umbracle/eth2-validator/issues/7)]
- feat: Aggreate and expose node metrics [[GH-29](https://github.com/umbracle/eth2-validator/issues/29)]
- feat: Add restart counter for task and mark failed [[GH-11](https://github.com/umbracle/eth2-validator/issues/11)]
- feat: Add logical and cli support to stop (as destroy) a container [[GH-3](https://github.com/umbracle/eth2-validator/issues/3)]
- feat: Add volume storage [[GH-1](https://github.com/umbracle/eth2-validator/issues/1)]
- feat: Add github ci workflow [[GH-10](https://github.com/umbracle/eth2-validator/issues/10)]

# 0.1.0 (12 May, 2022)

- Initial public release.
