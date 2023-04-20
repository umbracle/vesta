import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Ethereum mainnet

In this tutorial we are going to deploy a combination of [Beacon node and Execution client](https://ethereum.org/en/developers/docs/nodes-and-clients/) for the Ethereum mainnet chain.

Deploy an Execution client node:

````mdx-code-block
<Tabs>
<TabItem value="geth" label="Geth" default>

```bash
$ vesta deploy --type Geth --chain mainnet --alias el_node
```

</TabItem>
<TabItem value="nethermind" label="Nethermind">

```bash
$ vesta deploy --type Nethermind --chain mainnet --alias el_node
```

</TabItem>
<TabItem value="besu" label="Besu">

```bash
$ vesta deploy --type Besu --chain mainnet --alias el_node
```

</TabItem>
</Tabs>
````

Deploy a Beacon node that connects with the Execution client:

````mdx-code-block
<Tabs>
<TabItem value="prysm" label="Prysm" default>

```bash
$ vesta deploy --type Prysm --chain mainnet execution_node=el_node use_checkpoint=true
```

</TabItem>
<TabItem value="lighthouse" label="Lighthouse">

```bash
$ vesta deploy --type Lighthouse --chain mainnet execution_node=el_node
```

</TabItem>
<TabItem value="teku" label="Teku">

```bash
$ vesta deploy --type Teku --chain mainnet execution_node=el_node
```

</TabItem>
</Tabs>
````
