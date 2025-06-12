package contract

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// DecentralizedNamingSystem is a Go binding for the smart contract.
type DecentralizedNamingSystem struct {
	*bind.BoundContract
}

// ABI JSON for DecentralizedNamingSystem
const abiJSON = `[
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "indexed": false,
          "internalType": "string",
          "name": "cid",
          "type": "string"
        }
      ],
      "name": "CIDUpdated",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "owner",
          "type": "address"
        }
      ],
      "name": "NameRegistered",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "oldOwner",
          "type": "address"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "OwnershipTransferred",
      "type": "event"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "internalType": "string",
          "name": "cid",
          "type": "string"
        }
      ],
      "name": "register",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "internalType": "string",
          "name": "newCID",
          "type": "string"
        }
      ],
      "name": "updateCID",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "transferOwnership",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        }
      ],
      "name": "resolveCID",
      "outputs": [
        {
          "internalType": "string",
          "name": "",
          "type": "string"
        }
      ],
      "stateMutability": "view",
      "type": "function",
      "constant": true
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "name",
          "type": "string"
        }
      ],
      "name": "getOwner",
      "outputs": [
        {
          "internalType": "address",
          "name": "",
          "type": "address"
        }
      ],
      "stateMutability": "view",
      "type": "function",
      "constant": true
    }
  ]`

// NewDecentralizedNamingSystem creates a new instance of the contract binding.
func NewDecentralizedNamingSystem(address common.Address, backend bind.ContractBackend) (*DecentralizedNamingSystem, error) {
	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}

	// Create the bound contract
	contract := bind.NewBoundContract(address, parsedABI, backend, backend, backend)
	return &DecentralizedNamingSystem{BoundContract: contract}, nil
}

// Register calls the register function on the contract.
func (c *DecentralizedNamingSystem) Register(opts *bind.TransactOpts, name, cid string) (*types.Transaction, error) {
	return c.Transact(opts, "register", name, cid)
}

// ResolveCID calls the resolveCID function on the contract.
func (c *DecentralizedNamingSystem) ResolveCID(opts *bind.CallOpts, name string) (string, error) {
	var out []interface{}
	err := c.Call(opts, &out, "resolveCID", name)
	if err != nil {
		return "error", err
	}
	return out[0].(string), nil
}

// UpdateCID calls the updateCID function on the contract.
func (c *DecentralizedNamingSystem) UpdateCID(opts *bind.TransactOpts, name, newCID string) (*types.Transaction, error) {
	return c.Transact(opts, "updateCID", name, newCID)
}

// TransferOwnership calls the transferOwnership function on the contract.
func (c *DecentralizedNamingSystem) TransferOwnership(opts *bind.TransactOpts, name string, newOwner common.Address) (*types.Transaction, error) {
	return c.Transact(opts, "transferOwnership", name, newOwner)
}

// GetOwner calls the getOwner function on the contract.
func (c *DecentralizedNamingSystem) GetOwner(opts *bind.CallOpts, name string) (common.Address, error) {
	var out []interface{}
	err := c.Call(opts, &out, "getOwner", name)
	if err != nil {
		return common.Address{}, err
	}
	return out[0].(common.Address), nil
}
