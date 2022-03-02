package types

type VegaNode struct {
	HomeDir                string
	NodeWalletPassFilePath string
}

type TendermintNode struct {
	HomeDir         string
	GenesisFilePath string
}

type DataNode struct {
	HomeDir    string
	BinaryPath string
}

type NodeSet struct {
	Mode       string
	Vega       VegaNode
	Tendermint TendermintNode
	DataNode   *DataNode
}

type Wallet struct {
	HomeDir               string
	Network               string
	ServiceConfigFilePath string
	PublicKeyFilePath     string
	PrivateKeyFilePath    string
}

type Faucet struct {
	HomeDir            string
	PublicKey          string
	ConfigFilePath     string
	WalletFilePath     string
	WalletPassFilePath string
}

type GeneratedServices struct {
	Wallet   *Wallet
	Faucet   *Faucet
	NodeSets []NodeSet
}

type NetworkJobs struct {
	NodesSetsJobIDs []string
	ExtraJobIDs     []string
	FaucetJobID     string
	WalletJobID     string
}

const (
	NodeModeValidator           = "validator"
	NodeModeFull                = "full"
	NodeWalletChainTypeVega     = "vega"
	NodeWalletChainTypeEthereum = "ethereum"
)