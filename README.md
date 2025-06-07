# ipfs_gin_example

## 拉依赖
```cmd
go get github.com/gin-gonic/gin
go get github.com/dgraph-io/badger/v4
go get github.com/multiformats/go-multihash # Optional, for real CID, but we'll use hex hash for simplicity first
go get github.com/mitchellh/go-homedir # For database path
```
## 启动


启动ganache
```cmd
ganache-cli --port 8545 --networkId 1337 --chainId 1337
```
使用truffle部署智能合约
```cmd
truffle init
```
将合约文件放入contracts文件夹DecentralizedNamingSystem.sol
```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract DecentralizedNamingSystem {
    struct Record {
        string cid;
        address owner;
    }

    mapping(string => Record) private records;

    event NameRegistered(string indexed name, address indexed owner);
    event CIDUpdated(string indexed name, string cid);
    event OwnershipTransferred(string indexed name, address indexed oldOwner, address indexed newOwner);

    modifier onlyOwner(string memory name) {
        require(records[name].owner == msg.sender, "Not the owner of this name");
        _;
    }

    function register(string calldata name, string calldata cid) external {
        require(records[name].owner == address(0), "Name already registered");
        records[name] = Record({cid: cid, owner: msg.sender});
        emit NameRegistered(name, msg.sender);
    }

    function updateCID(string calldata name, string calldata newCID) external onlyOwner(name) {
        records[name] = Record({cid: newCID, owner: msg.sender});
        emit CIDUpdated(name, newCID);
    }

    function transferOwnership(string calldata name, address newOwner) external onlyOwner(name) {
        require(newOwner != address(0), "New owner is zero address");
        address oldOwner = records[name].owner;
        records[name].owner = newOwner;
        emit OwnershipTransferred(name, oldOwner, newOwner);
    }

    function resolveCID(string calldata name) external view returns (string memory) {
        require(records[name].owner != address(0), "Name not registered");
        return records[name].cid;
    }

    function getOwner(string calldata name) external view returns (address) {
        return records[name].owner;
    }
}
```
配置truffle-config.js
```js
module.exports = {
  networks: {
	  development: {
		host: "127.0.0.1",
		port: 8545,
		network_id: "*",
		gas: 6721975, // Match Ganache block gas limit
		gasPrice: 20000000000, // 20 Gwei
	  },
	},
  compilers: {
	  solc: {
		version: "0.8.0",
	  },
	},
};
```
创建脚本文件2_deploy_contracts.js于migrations下
```js
const DecentralizedNamingSystem = artifacts.require("DecentralizedNamingSystem");

module.exports = function (deployer) {
  deployer.deploy(DecentralizedNamingSystem, { gas: 5000000 });
};
```
开始编译
```cmd
truffle compile
truffle migrate --reset
```
设置以下环境变量
```cmd
set CONTRACT_ADDRESS=0xYourContractAddress
set PRIVATE_KEY=0xYourPrivateKey
set CHAIN_ID=1337

go build
go run main.go
```



## 测试

1. 上传文件(test目录下), 数据存储在data下面

   ```cmd
   先注释掉mock.go mockMap中的hello字段，执行
   curl.exe -X PUT "http://localhost:8080/hello.com/home/hello.txt" --data-binary "@hello.txt"
   执行后解掉domain的hello字段的注释并修改cid为返回的新cid
   ```

   

2. 下载文件

   ```cmd
   curl.exe "http://localhost:8080/hello.com/home/hello.txt" --output download_hello.txt
   ```

   