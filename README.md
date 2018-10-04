Ethereum AnchorMaker
=============

This version of AnchorMaker is a program that runs against factomd and geth nodes, listening to the Factom and Ethereum networks respecively, and submits rolling 1000 block windows of Directory Block anchors from the former into the latter. Once an anchor is confirmed as having been included in a block on Ethereum, an AnchorRecord will be submitted back to Factom as a "receipt" of sorts for the anchor --- this is what allows for factomd to return anchor related information in the "anchors-by-height" call.

If a backlog of unanchored Directory Blocks exist, AnchorMaker will start by submitting an anchor for DBlock 999 or the current head DBlock found in factomd (whichever is lower). Then, it will proceed to work through any remaining 1000 block windows of backlogged Directory Blocks until it is all caught up.

Instructions
--------

### Setting up the environment
Make sure that your factomd node is up to date, currently running, and fully synced (instructions sold separately). I prefer to keep these programs in their own tmux sessions, but you could just put everything in separate terminal windows if you're running locally.

**Note: Because this program will automatically write anchor records to the anchor chain [(Name: FactomEthereumAnchorChain, ChainID: 6e4540d08d5ac6a1a394e982fb6a2ab8b516ee751c37420055141b94fe070bfe)](http://explorer.factom.com/chain/6e4540d08d5ac6a1a394e982fb6a2ab8b516ee751c37420055141b94fe070bfe), it is recommended (pretty please) that you run AnchorMaker with a simulated Factom network or similar, rather than on the live main-net.**


First, download the anchormaker-eth repo and copy the included anchormaker.conf file to your .factom directory:
```
cd $GOPATH/src/github.com/FactomProject
git clone git@github.com:FactomProject/anchormaker-eth.git
cd anchormaker-eth
cp anchormaker.conf $HOME/.factom/anchormaker.conf
```

Then install anchormaker from the repo's root directory with `go install` (or just do a `go build` to keep the binary contained to the repo folder)

Now you'll want to download and install the geth Ethereum client:
- [Github repo](https://github.com/ethereum/go-ethereum)
- [Installation instructions](https://github.com/ethereum/go-ethereum/wiki/Installation-Instructions-for-Ubuntu)


Next, let's get geth fully synced. Open up a tmux session and navigate to the anchormaker-eth/ethereum folder first --- that's where the compiled solidity contract is. Then issue one of the following to start geth:

- main-net:
`geth --cache=4096 --verbosity 3 --rpc --rpcapi "personal,net,web3,eth" console 2>> ~/ethlogs/eth.log`

- ropsten test-net:
`geth --testnet --cache=4096 --verbosity 3 --rpc --rpcapi "personal,net,web3,eth" console 2>> ~/ethlogs/eth.log`

Geth will run with `--syncmode "fast"` by default, which is essentially the concept of a "full" node in Ethereum. It will quickly sync the block headers up to 64 blocks before current head block, then begin processing all transactions and building up the state. This can still take a **really long** time, despite the name "fast". So for testing purposes, just running a light node will be sufficient. This will allow your node to sync very quickly and have little overhead. Simply add the following flag: `--syncmode "light"`

To check if your geth node is syncing, issue `eth.syncing` from within the geth console. A response of `false` means your node is fully synced and you are safe to proceed.

Once fully synced, create an Ethereum address and put it in the anchormaker.conf file along with the password you used for it. Also, you'll want to change the `WalletKeyPath` field to be either the proper keystore filepath for your system. It'll be a file in the `/home/USER/.ethereum/testnet/keystore/...` directory.

For test-net, just issue the following from the geth console to import a key we've already used and funded in the past:
```javascript
web3.personal.importRawKey("db8a08597d0dfa5cc4884117546c2c6f069c34ce8a2eba1015920f12f1088a1b","aStrongPassword")
```

That should return the address `0x7b2d985524c3fadc0f939342fc95edcaf7163616` afterwards. You can check it has been imported by typing `eth.accounts`. Make sure the address has been funded, 1 ETH should be good enough to start. There are plenty of faucets for ropsten test-net ethers, such as https://faucet.ropsten.be/

Once the address balance is non-zero, you will be able to move onto deployment of the FactomAnchor smart contract.

### Deploying New FactomAnchor Smart Contract

Assuming you started geth within the anchormaker-eth/ethereum directory/ you should be able to just copy and paste this block of commands into the geth console to submit a transaction containing a new contract instance.
```javascript
web3.personal.unlockAccount(eth.accounts[0], "aStrongPassword")
loadScript('anchorCompiled.js')
var anchorContractAbi = anchorOutput.contracts["anchorContract.sol:FactomAnchor"].abi;
var anchorContract = eth.contract(JSON.parse(anchorContractAbi))
var anchorBinCode = "0x" + anchorOutput.contracts['anchorContract.sol:FactomAnchor'].bin
var deployTxObject = { from: eth.accounts[0], data: anchorBinCode, gas: 1000000 };
var anchorInstance = anchorContract.new(deployTxObject)
```

Now you'll wait for the transaction to be confirmed. You can watch your address for pending transactions at etherscan.io / ropsten.etherscan.io, or you can keep trying the following command in the geth console every 30 seconds or so:

```javascript
eth.getTransactionReceipt(anchorInstance.transactionHash).contractAddress
```

When there is no error returned, and just a contract address as a string, that's how you know it has been confirmed and the contract is now deployed. Update the anchormaker.conf file with the contract address you'll be using.

To give the contract an small initial test, you can interact with it from the geth console like so:
```javascript
var anchor = anchorContract.at(eth.getTransactionReceipt(anchorInstance.transactionHash).contractAddress)
anchor.getAnchor(0)
anchor.checkFrozen()
```

Now you can rename the tmux session (if you're using one) by pressing `<ctrl+b>` then `$`, then type `geth` and press `enter`. You'll use this name if you want to open the session again and interact with the geth console. Press `<ctrl+b>` then `d` to detach the session, and you can move on to running anchormaker. 

### Running AnchorMaker

Now that all your nodes are synced and you have a fresh contract deployed, make sure the information in the anchormaker.conf file reflects your current setup.

Generate a new EC address (```factom-cli newecaddress``` should be sufficient assuming you have the tools), or use an existing one you have funded already. Then modify the $HOME/.factom/anchormaker.conf file and change the "ServerECKey" to the keys we just created.

Open a new tmux session, and run ```anchormaker > anchorlog.txt``` or ```./anchormaker > anchorlog.txt``` depending on whether you issued ```go install``` or ```go build``` previously. Now you'll be able to detach the session (<ctrl+b> then d), and run `tail -f anchorlog.txt` to follow the program's output.