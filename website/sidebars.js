/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  tutorialSidebar: [
    'getting-started',
    'installation',
    'alternatives',
    'dependencies',
    {
      type: 'category',
      label: 'Concepts',
      items: [
        'concepts/plugins',
        'concepts/scheduler',
        'concepts/telemetry'
      ],
    },
    {
      type: 'category',
      label: 'Command Line Interface',
      items: [
        'cli/server',
        'cli/deploy',
        'cli/destroy',
        'cli/deployment-list',
        'cli/deployment-status',
        'cli/catalog-list',
        'cli/catalog-inspect',
      ],
    },
    {
      type: 'category',
      label: 'Plugins',
      items: [
        'plugins/overview',
        'plugins/besu',
        'plugins/geth',
        'plugins/lighthouse',
        'plugins/nethermind',
        'plugins/prysm',
        'plugins/teku',
      ],
    },
    {
      type: "category",
      label: "Use cases",
      items: [
        "use-cases/node-as-a-service",
        "use-cases/validator-stack"
      ]
    },
    {
      type: 'category',
      label: 'Tutorials',
      items: [
        'tutorials/ethereum_mainnet',
      ]
    }
  ],
};

module.exports = sidebars;
