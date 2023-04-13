package catalog

import "github.com/umbracle/vesta/internal/framework"

var Catalog = map[string]framework.Framework{
	"lighthouse": &Lighthouse{},
	"prysm":      &Prysm{},
	"teku":       &Teku{},
	"besu":       &Besu{},
	"geth":       &Geth{},
	"nethermind": &Netermind{},
}

var jwtToken = "04592280e1778419b7aa954d43871cb2cfb2ebda754fb735e8adeb293a88f9bf"

var (
	goerliChain  = "goerli"
	sepoliaChain = "sepolia"
	mainnetChain = "mainnet"
)

func newTestingFramework(chain string) *framework.TestingFramework {
	fr := &framework.TestingFramework{
		F: Catalog[chain],
	}
	return fr
}

func getCheckpointURL(chain string) string {
	return "https://" + chain + ".checkpoint-sync.ethpandaops.io"
}
