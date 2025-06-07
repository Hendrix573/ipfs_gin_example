package contract

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client manages interactions with the DecentralizedNamingSystem smart contract.
type Client struct {
	client   *ethclient.Client
	contract *DecentralizedNamingSystem
}

// NewClient initializes a new contract client.
func NewClient(rpcURL, contractAddress string) (*Client, error) {
	// Connect to Ethereum node
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	// Initialize contract instance
	contract, err := NewDecentralizedNamingSystem(common.HexToAddress(contractAddress), client)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &Client{
		client:   client,
		contract: contract,
	}, nil
}

// Close closes the Ethereum client connection.
func (c *Client) Close() {
	c.client.Close()
}

// RegisterName registers a name and CID in the smart contract.
func (c *Client) RegisterName(auth *bind.TransactOpts, name, cid string) error {
	tx, err := c.contract.Register(auth, name, cid)
	if err != nil {
		return err
	}
	// Wait for transaction to be mined
	_, err = bind.WaitMined(context.Background(), c.client, tx)
	return err
}

// ResolveCID resolves a name to its CID.
func (c *Client) ResolveCID(name string) (string, error) {
	return c.contract.ResolveCID(&bind.CallOpts{}, name)
}

// UpdateCID updates the CID for a name in the smart contract.
func (c *Client) UpdateCID(auth *bind.TransactOpts, name, newCID string) error {
	tx, err := c.contract.UpdateCID(auth, name, newCID)
	if err != nil {
		return err
	}
	_, err = bind.WaitMined(context.Background(), c.client, tx)
	return err
}

// TransferOwnership transfers ownership of a name to another address.
func (c *Client) TransferOwnership(auth *bind.TransactOpts, name string, newOwner common.Address) error {
	tx, err := c.contract.TransferOwnership(auth, name, newOwner)
	if err != nil {
		return err
	}
	_, err = bind.WaitMined(context.Background(), c.client, tx)
	return err
}

// GetOwner retrieves the owner of a name.
func (c *Client) GetOwner(name string) (common.Address, error) {
	return c.contract.GetOwner(&bind.CallOpts{}, name)
}
