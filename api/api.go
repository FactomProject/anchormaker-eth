package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/FactomProject/factom"

	"github.com/FactomProject/factomd/common/adminBlock"
	"github.com/FactomProject/factomd/common/directoryBlock"
	"github.com/FactomProject/factomd/common/entryBlock"
	"github.com/FactomProject/factomd/common/entryCreditBlock"
	"github.com/FactomProject/factomd/common/factoid"
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/primitives"
)

var server string = "localhost:8088" //Localhost
//var server string = "52.17.183.121:8088" //TestNet
//var server string = "52.18.72.212:8088" //MainNet

func SetServer(serverAddress string) {
	server = serverAddress
	factom.SetFactomdServer(serverAddress)
}

func GetECBalance(ecPublicKey string) (int64, error) {
	ecAddress, err := factoid.PublicKeyStringToECAddressString(ecPublicKey)
	if err != nil {
		return 0, err
	}

	balance, err := factom.GetECBalance(ecAddress)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func GetFactoidBalance(factoidPublicKey string) (int64, error) {
	fAddress, err := factoid.PublicKeyStringToFactoidAddressString(factoidPublicKey)
	if err != nil {
		return 0, err
	}

	balance, err := factom.GetFactoidBalance(fAddress)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

type DBlockHead struct {
	KeyMR string
}

func GetDBlock(keymr string) (interfaces.IDirectoryBlock, error) {
	raw, err := GetRaw(keymr)
	if err != nil {
		return nil, err
	}
	dblock, err := directoryBlock.UnmarshalDBlock(raw)
	if err != nil {
		return nil, err
	}
	return dblock, nil
}

func GetDBlockByHeight(height uint32) (interfaces.IDirectoryBlock, error) {
	resp, err := factom.GetDBlockByHeight(int64(height))
	if err != nil {
		if err.Error() == "Block not found" {
			return nil, nil
		}
		return nil, err
	}
	raw, err := hex.DecodeString(resp.RawData)
	if err != nil {
		return nil, err
	}
	dblock, err := directoryBlock.UnmarshalDBlock(raw)
	if err != nil {
		return nil, err
	}
	return dblock, nil
}

// TODO: move calculation of the Merkle root for a window of blocks to a new function in factomd/primitives package when creating factomd "anchor" RPC call
// GetMerkleRootOfDBlockWindow calculates a Merkle root for all Directory Blocks from height to (height - size + 1)
func GetMerkleRootOfDBlockWindow(height, size uint32) (interfaces.IHash, error) {
	to := height
	var from uint32
	if to < (size - 1) {
		from = 0
	} else {
		from = height - size + 1
	}
	var dblockMRs []interfaces.IHash
	for i := from; i <= to; i++ {
		block, err := GetDBlockByHeight(uint32(i))
		if err != nil {
			return nil, err
		}
		dblockMR, err := primitives.NewShaHashFromStr(block.BodyKeyMR().String())
		if err != nil {
			return nil, err
		}
		dblockMRs = append(dblockMRs, dblockMR)
	}
	if from == to {
		// Only one DBlock in range, just return it's KeyMR
		return dblockMRs[0], nil
	}
	branch := primitives.BuildMerkleBranchForEntryHash(dblockMRs, dblockMRs[0], true)
	merkleRoot := branch[len(branch) - 1].Top
	return merkleRoot, nil
}

func GetABlock(keymr string) (interfaces.IAdminBlock, error) {
	raw, err := GetRaw(keymr)
	if err != nil {
		return nil, err
	}
	block, err := adminBlock.UnmarshalABlock(raw)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GetECBlock(keymr string) (interfaces.IEntryCreditBlock, error) {
	raw, err := GetRaw(keymr)
	if err != nil {
		return nil, err
	}
	block, err := entryCreditBlock.UnmarshalECBlock(raw)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GetFBlock(keymr string) (interfaces.IFBlock, error) {
	raw, err := GetRaw(keymr)
	if err != nil {
		return nil, err
	}
	block, err := factoid.UnmarshalFBlock(raw)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GetEBlock(keymr string) (interfaces.IEntryBlock, error) {
	raw, err := GetRaw(keymr)
	if err != nil {
		return nil, err
	}
	block, err := entryBlock.UnmarshalEBlock(raw)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GetEntry(hash string) (interfaces.IEBEntry, error) {
	raw, err := GetRaw(hash)
	if err != nil {
		return nil, err
	}
	entry, err := entryBlock.UnmarshalEntry(raw)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func GetDBlockHead() (string, error) {
	//return "3a5ec711a1dc1c6e463b0c0344560f830eb0b56e42def141cb423b0d8487a1dc", nil //10
	//return "cde346e7ed87957edfd68c432c984f35596f29c7d23de6f279351cddecd5dc66", nil //100
	//return "d13472838f0156a8773d78af137ca507c91caf7bf3b73124d6b09ebb0a98e4d9", nil //200

	return factom.GetDBlockHead()

	resp, err := http.Get(fmt.Sprintf("http://%s/v1/directory-block-head/", server))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf(string(body))
	}

	d := new(DBlockHead)
	json.Unmarshal(body, d)

	return d.KeyMR, nil
}

type Data struct {
	Data string
}

func GetRaw(keymr string) ([]byte, error) {
	return factom.GetRaw(keymr)

	fmt.Printf("GetRaw %v\n", keymr)
	resp, err := http.Get(fmt.Sprintf("http://%s/v1/get-raw-data/%s", server, keymr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(string(body))
	}

	d := new(Data)
	if err := json.Unmarshal(body, d); err != nil {
		return nil, err
	}

	raw, err := hex.DecodeString(d.Data)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
