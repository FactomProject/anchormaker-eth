package factom

import (
	"fmt"
	"time"

	"github.com/FactomProject/anchormaker-eth/api"
	"github.com/FactomProject/anchormaker-eth/config"
	"github.com/FactomProject/anchormaker-eth/database"
	"github.com/FactomProject/factom"
	"github.com/FactomProject/factomd/anchor"
	"github.com/FactomProject/factomd/common/entryBlock"
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/primitives"
)

var IgnoreWrongEntries = true
var WindowSize uint32
var AnchorSigPublicKeys []interfaces.Verifier
var ServerPrivKey *primitives.PrivateKey
var ECAddress *factom.ECAddress

// 6e4540d08d5ac6a1a394e982fb6a2ab8b516ee751c37420055141b94fe070bfe
var EthereumAnchorChainID interfaces.IHash
var FirstEthereumAnchorChainEntryHash interfaces.IHash

func init() {
	e := CreateFirstEthereumAnchorEntry()
	EthereumAnchorChainID = e.ChainID
	FirstEthereumAnchorChainEntryHash = e.GetHash()
}

func LoadConfig(c *config.AnchorConfig) {
	ecAddress, err := factom.GetECAddress(c.App.ECPrivateKey)
	if err != nil {
		panic(err)
	}
	ECAddress = ecAddress

	// All previously used Anchor Record keys for validation
	for _, v := range c.App.AllAnchorRecordPublicKeys {
		pubKey := new(primitives.PublicKey)
		err := pubKey.UnmarshalText([]byte(v))
		if err != nil {
			panic(err)
		}
		AnchorSigPublicKeys = append(AnchorSigPublicKeys, pubKey)
	}

	// Current private key we'll use to sign Anchor Records
	key, err := primitives.NewPrivateKeyFromHex(c.App.CurrentAnchorRecordPrivateKey)
	if err != nil {
		panic(err)
	}
	ServerPrivKey = key
	AnchorSigPublicKeys = append(AnchorSigPublicKeys, ServerPrivKey.Pub)

	WindowSize = c.App.WindowSize
}

// SynchronizeFactomData checks for recently created directory blocks and returns how many new ones were found
func SynchronizeFactomData(dbo *database.AnchorDatabaseOverlay) (int, error) {
	fmt.Println("\nSynchronizeFactomData():")
	blockCount := 0
	ps, err := dbo.FetchProgramState()
	if err != nil {
		return 0, err
	}
	//note, this mutex could probably be reworked to prevent a short time span of a race here between fetch and lock
	ps.ProgramStateMutex.Lock()
	defer ps.ProgramStateMutex.Unlock()

	// If it's 0, we don't know if we have ANY blocks
	nextHeight := ps.LastFactomDBlockHeightChecked
	if nextHeight > 0 {
		// It's more than 0, so we know we have that block --> skip it
		nextHeight++
	}

	var dBlockList []interfaces.IDirectoryBlock
	for {
		dBlock, err := api.GetDBlockByHeight(nextHeight)
		if err != nil {
			return 0, err
		}
		if dBlock == nil {
			break
		}

		dBlockList = append(dBlockList, dBlock)
		fmt.Printf("Fetched newly found directory block %v\n", dBlock.GetDatabaseHeight())
		nextHeight = dBlock.GetDatabaseHeight() + 1
	}
	if len(dBlockList) == 0 {
		return 0, nil
	}

	var currentHeadHeight uint32 = 0
	for _, dBlock := range dBlockList {
		for _, v := range dBlock.GetDBEntries() {
			// Looking for Ethereum anchor records in new DBlock
			if v.GetChainID().String() != EthereumAnchorChainID.String() {
				continue
			}

			entryBlock, err := api.GetEBlock(v.GetKeyMR().String())
			if err != nil {
				return 0, err
			}
			for _, eh := range entryBlock.GetEntryHashes() {
				if eh.IsMinuteMarker() || eh.String() == FirstEthereumAnchorChainEntryHash.String() {
					continue
				}

				entry, err := api.GetEntry(eh.String())
				if err != nil {
					return 0, err
				}

				ar, valid, _ := anchor.UnmarshalAndValidateAnchorEntryAnyVersion(entry, AnchorSigPublicKeys)
				if !valid {
					fmt.Printf("Invalid anchor: %v\n", entry)
					continue
				}

				anchorData, err := dbo.FetchAnchorData(ar.DBHeightMax)
				if err != nil {
					return 0, err
				}
				if anchorData.MerkleRoot == "" {
					// Calculate Merkle root that should be in the anchor record
					merkleRoot, err := api.GetMerkleRootOfDBlockWindow(ar.DBHeightMax, WindowSize)
					if err != nil {
						return 0, err
					}
					anchorData.MerkleRoot = merkleRoot.String()
				}
				if anchorData.MerkleRoot != ar.WindowMR {
					if IgnoreWrongEntries == false {
						fmt.Printf("%v, %v\n", ar.DBHeightMax, anchorData)
						panic(fmt.Sprintf("%v vs %v", anchorData.MerkleRoot, ar.WindowMR))
						return 0, fmt.Errorf("AnchorData MerkleRoot does not match AnchorRecord MerkleRoot")
					}
					fmt.Printf("Bad AR: Height %v has MerkleRoot %v in database, but found %v in AnchorRecord on Factom\n", ar.DBHeightMax, anchorData.MerkleRoot, ar.WindowMR)
					continue
				}

				if ar.Ethereum != nil {
					fmt.Printf("Found Ethereum anchor record in Factom DBlock %v: %v, %v\n", dBlock.GetDatabaseHeight(), ar.DBHeightMax, ar.WindowMR)
					anchorData.Ethereum.ContractAddress = ar.Ethereum.ContractAddress
					anchorData.Ethereum.TxID = ar.Ethereum.TxID
					anchorData.Ethereum.BlockHeight = ar.Ethereum.BlockHeight
					anchorData.Ethereum.BlockHash = ar.Ethereum.BlockHash
					anchorData.Ethereum.TxIndex = ar.Ethereum.TxIndex
					anchorData.EthereumRecordHeight = dBlock.GetDatabaseHeight()
					anchorData.EthereumRecordEntryHash = eh.String()

					if ps.LastConfirmedAnchorDBlockHeight < anchorData.DBlockHeight {
						ps.LastConfirmedAnchorDBlockHeight = anchorData.DBlockHeight
					}
				}

				err = dbo.InsertAnchorData(anchorData, false)
				if err != nil {
					return 0, err
				}
				blockCount++
			}
		}

		// Updating new directory blocks
		anchorData, err := dbo.FetchAnchorData(dBlock.GetDatabaseHeight())
		if err != nil {
			return 0, err
		}
		if anchorData == nil {
			anchorData := new(database.AnchorData)
			anchorData.DBlockHeight = dBlock.GetDatabaseHeight()
			err = dbo.InsertAnchorData(anchorData, false)
			if err != nil {
				return 0, err
			}
			blockCount++
		}
		currentHeadHeight = dBlock.GetDatabaseHeight()
		ps.LastFactomDBlockHeightChecked = currentHeadHeight

		err = dbo.InsertProgramState(ps)
		if err != nil {
			return 0, err
		}
	}

	err = dbo.UpdateAnchorDataHead()
	if err != nil {
		return 0, err
	}

	return blockCount, nil
}

// SaveAnchorsIntoFactom submits Factom entries (anchor records) for all newly confirmed contract txs found during the SynchronizationLoop
func SaveAnchorsIntoFactom(dbo *database.AnchorDatabaseOverlay) error {
	fmt.Println("\nSaveAnchorsIntoFactom():")

	ecBalance, err := factom.GetECBalance(ECAddress.PubString())
	if err != nil {
		fmt.Println(err.Error())
	} else if ecBalance == 0 {
		fmt.Printf("EC Balance: 0\nCannot submit anchors. Returning.")
		return nil
	} else {
		fmt.Printf("EC Balance: %d\n", ecBalance)
	}

	ps, err := dbo.FetchProgramState()
	if err != nil {
		return err
	}
	// This mutex could probably be reworked to prevent a short time span of a race here between fetch and lock
	ps.ProgramStateMutex.Lock()
	defer ps.ProgramStateMutex.Unlock()

	anchorData, err := dbo.FetchAnchorDataHead()
	if err != nil {
		return err
	}
	if anchorData == nil {
		anchorData, err = dbo.FetchAnchorData(0)
		if err != nil {
			return err
		}
		if anchorData == nil {
			// nothing found
			return nil
		}
	}

	// Try to submit as many as 10 anchor records/receipts into Factom
	for i := 0; i < 10; {
		// Check that the anchor tx was confirmed on Ethereum, and that we haven't recorded that tx's receipt in Factom yet
		if anchorData.Ethereum.BlockHash != "" && anchorData.EthereumRecordEntryHash == "" {
			anchorRecord := new(anchor.AnchorRecord)
			anchorRecord.AnchorRecordVer = 2
			anchorRecord.DBHeightMax = anchorData.DBlockHeight
			anchorRecord.DBHeightMin = anchorData.DBlockHeight - WindowSize + 1
			anchorRecord.WindowMR = anchorData.MerkleRoot
			anchorRecord.RecordHeight = ps.LastFactomDBlockHeightChecked + 1

			anchorRecord.Ethereum = new(anchor.EthereumStruct)
			anchorRecord.Ethereum.ContractAddress = anchorData.Ethereum.ContractAddress
			anchorRecord.Ethereum.TxID = anchorData.Ethereum.TxID
			anchorRecord.Ethereum.BlockHeight = anchorData.Ethereum.BlockHeight
			anchorRecord.Ethereum.BlockHash = anchorData.Ethereum.BlockHash
			anchorRecord.Ethereum.TxIndex = anchorData.Ethereum.TxIndex

			tx, err := CreateAndSendAnchor(anchorRecord)
			if err != nil {
				return err
			}
			anchorData.EthereumRecordEntryHash = tx

			// Resetting AnchorRecord
			anchorRecord.Ethereum = nil

			err = dbo.InsertAnchorData(anchorData, false)
			if err != nil {
				fmt.Println("error in InsertAnchorData")
				return err
			}
			i++
		}
		anchorData, err = dbo.FetchAnchorData(anchorData.DBlockHeight + 1)
		if err != nil {
			fmt.Println("error in FetchAnchorData")
			return err
		}
		if anchorData == nil {
			fmt.Println("error anchordata is nil")
			break
		}
	}
	return nil
}

// CreateAndSendAnchor submits the anchor record entry to the Factom network and returns the txID
func CreateAndSendAnchor(ar *anchor.AnchorRecord) (string, error) {
	fmt.Printf("Sending anchor record to Factom: %v\n", ar)
	if ar.Ethereum != nil {
		entry, err := CreateAnchorEntry(ar, EthereumAnchorChainID, ServerPrivKey)
		if err != nil {
			return "", err
		}
		_, txID, err := JustFactomize(entry)
		if err != nil {
			return "", err
		}
		return txID, nil
	}
	return "", nil
}

// CreateAnchorEntry constructs and returns a new entry with the anchor record as the Content
// and the server's signature of the anchor record as the only External ID
func CreateAnchorEntry(aRecord *anchor.AnchorRecord, chainID interfaces.IHash, serverPrivKey *primitives.PrivateKey) (*entryBlock.Entry, error) {
	record, sig, err := aRecord.MarshalAndSignV2(ServerPrivKey)
	if err != nil {
		return nil, err
	}

	entry := new(entryBlock.Entry)
	entry.ChainID = chainID
	entry.Content = primitives.ByteSlice{Bytes: record}
	entry.ExtIDs = []primitives.ByteSlice{primitives.ByteSlice{Bytes: sig}}

	return entry, nil
}

// JustFactomizeChain creates and submits a new chain using the given EntryBlock Entry and returns the txID of the commit and reveal
func JustFactomizeChain(entry *entryBlock.Entry) (string, string, error) {
	//Convert entryBlock Entry into factom Entry
	//fmt.Printf("Entry - %v\n", entry)
	j, err := entry.JSONByte()
	if err != nil {
		return "", "", err
	}
	e := new(factom.Entry)
	err = e.UnmarshalJSON(j)
	if err != nil {
		return "", "", err
	}

	chain := factom.NewChain(e)

	// Commit and reveal
	tx1, err := factom.CommitChain(chain, ECAddress)
	if err != nil {
		return "", "", fmt.Errorf("chain commit error: %v", err)
	}
	time.Sleep(10 * time.Second)
	tx2, err := factom.RevealChain(chain)
	if err != nil {
		return "", "", fmt.Errorf("chain reveal error: %v", err)
	}
	return tx1, tx2, nil
}

// JustFactomize creates and submits a new entry using the given EntryBlock Entry and returns the txID of the commit and reveal
func JustFactomize(entry *entryBlock.Entry) (string, string, error) {
	//Convert entryBlock Entry into factom Entry
	//fmt.Printf("Entry - %v\n", entry)
	j, err := entry.JSONByte()
	if err != nil {
		return "", "", err
	}
	e := new(factom.Entry)
	err = e.UnmarshalJSON(j)
	if err != nil {
		return "", "", err
	}

	// Commit and reveal
	tx1, err := factom.CommitEntry(e, ECAddress)
	if err != nil {
		return "", "", fmt.Errorf("entry commit error: %v", err)
	}
	time.Sleep(3 * time.Second)
	tx2, err := factom.RevealEntry(e)
	if err != nil {
		return "", "", fmt.Errorf("entry reveal error: %v", err)
	}
	return tx1, tx2, nil
}

// CreateFirstEthereumAnchorEntry creates and returns the first entry in the Ethereum Anchor Chain
// but does not submit it to the Factom network
func CreateFirstEthereumAnchorEntry() *entryBlock.Entry {
	answer := new(entryBlock.Entry)

	answer.Version = 0
	answer.ExtIDs = []primitives.ByteSlice{primitives.ByteSlice{Bytes: []byte("FactomEthereumAnchorChain")}}
	answer.Content = primitives.ByteSlice{Bytes: []byte("This is the Factom Ethereum anchor chain, which records the anchors Factom puts on the Ethereum network.\n")}
	answer.ChainID = entryBlock.NewChainID(answer)

	return answer
}
