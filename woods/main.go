package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"github.com/taurusgroup/frost-ed25519/pkg/eddsa"
	"github.com/taurusgroup/frost-ed25519/pkg/frost"
	"github.com/taurusgroup/frost-ed25519/pkg/frost/keygen"
	"github.com/taurusgroup/frost-ed25519/pkg/frost/party"
	"github.com/taurusgroup/frost-ed25519/pkg/frost/sign"
	"github.com/taurusgroup/frost-ed25519/pkg/helpers"
	"github.com/taurusgroup/frost-ed25519/pkg/ristretto"
	"github.com/taurusgroup/frost-ed25519/pkg/state"
	"io/ioutil"
	"os"
	"path/filepath"
)

const maxN = 100

func usage() {
	cmd := filepath.Base(os.Args[0])
	fmt.Printf("usage: %v t n\nwhere 0 < t < n < %v\n", cmd, maxN)
}
func keygenDemo(t int, n int) {

	var err error
	if (n > maxN) || (t >= n) {
		usage()
		return
	}

	partyIDs := helpers.GenerateSet(party.ID(n))

	// structure holding parties' state and output
	states := map[party.ID]*state.State{}
	outputs := map[party.ID]*keygen.Output{}

	// create a state for each party
	for _, id := range partyIDs {
		states[id], outputs[id], err = frost.NewKeygenState(id, partyIDs, party.Size(t), 0)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	msgsOut1 := make([][]byte, 0, n)
	msgsOut2 := make([][]byte, 0, n*(n-1)/2)

	for _, s := range states {
		msgs1, err := helpers.PartyRoutine(nil, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut1 = append(msgsOut1, msgs1...)
	}

	for _, s := range states {
		msgs2, err := helpers.PartyRoutine(msgsOut1, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut2 = append(msgsOut2, msgs2...)
	}

	for _, s := range states {
		_, err := helpers.PartyRoutine(msgsOut2, s)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Get the public data
	fmt.Println("Group Key:")
	id0 := partyIDs[0]
	if err = states[id0].WaitForError(); err != nil {
		fmt.Println(err)
		return
	}
	public := outputs[id0].Public
	secrets := make(map[party.ID]*eddsa.SecretShare, n)
	groupKey := public.GroupKey
	fmt.Printf("  %x\n\n", groupKey.ToEd25519())

	for _, id := range partyIDs {
		if err := states[id].WaitForError(); err != nil {
			fmt.Println(err)
			return
		}
		shareSecret := outputs[id].SecretKey
		sharePublic := public.Shares[id]
		secrets[id] = shareSecret
		fmt.Printf("Party %d:\n  secret: %x\n  public: %x\n", id, shareSecret.Secret.Bytes(), sharePublic.Bytes())
	}

	// TODO: write JSON file, to take as input by CLI signer
	type KeyGenOutput struct {
		Secrets map[party.ID]*eddsa.SecretShare
		Shares  *eddsa.Public
	}

	kgOutput := KeyGenOutput{
		Secrets: secrets,
		Shares:  public,
	}

	var jsonData []byte
	jsonData, err = json.MarshalIndent(kgOutput, "", " ")
	if err != nil {
		fmt.Println(err)
		return
	}

	filename := "./keygenout.json"

	_ = ioutil.WriteFile(filename, jsonData, 0644)

	fmt.Printf("Success: output written to %v\n", filename)
}

func writeJSONToFile(filename string, data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = ioutil.WriteFile(filename, jsonData, 0644)
}

func verifyKeys(filename string, msg string) {

	message := []byte(msg)

	var err error

	type KeyGenOutput struct {
		Secrets map[party.ID]*eddsa.SecretShare
		Shares  *eddsa.Public
	}

	var kgOutput KeyGenOutput

	var jsonData []byte
	jsonData, err = ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = json.Unmarshal(jsonData, &kgOutput)
	if err != nil {
		fmt.Println(err)
		return
	}

	// get n and t from the keygen output
	var n party.Size
	var t party.Size

	n = kgOutput.Shares.PartyIDs.N()
	t = kgOutput.Shares.Threshold

	fmt.Printf("(t, n) = (%v, %v)\n", t, n)

	partyIDs := helpers.GenerateSet(n)

	secretShares := kgOutput.Secrets
	publicShares := kgOutput.Shares

	// structure holding parties' state and output
	states := map[party.ID]*state.State{}
	outputs := map[party.ID]*sign.Output{}

	msgsOut1 := make([][]byte, 0, n)
	msgsOut2 := make([][]byte, 0, n)

	for _, id := range partyIDs {
		states[id], outputs[id], err = frost.NewSignState(partyIDs, secretShares[id], publicShares, message, 0)
		if err != nil {
			fmt.Println()
		}
	}

	pk := publicShares.GroupKey

	for _, s := range states {
		msgs1, err := helpers.PartyRoutine(nil, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut1 = append(msgsOut1, msgs1...)
	}

	for _, s := range states {
		msgs2, err := helpers.PartyRoutine(msgsOut1, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut2 = append(msgsOut2, msgs2...)
	}

	for _, s := range states {
		_, err := helpers.PartyRoutine(msgsOut2, s)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	id0 := partyIDs[0]
	sig := outputs[id0].Signature
	if sig == nil {
		fmt.Println("null signature")
		return
	}

	if !ed25519.Verify(pk.ToEd25519(), message, sig.ToEd25519()) {
		fmt.Println("signature verification failed (ed25519)")
		return
	}

	if !pk.Verify(message, sig) {
		fmt.Println("signature verification failed")
		return
	}

	fmt.Printf("Success: signature is\nr: %x\ns: %x\n", sig.R.Bytes(), sig.S.Bytes())
}

// 拆分为多份文件
func keygenDemoV2(t int, n int) {

	var err error
	if (n > maxN) || (t >= n) {
		usage()
		return
	}

	partyIDs := helpers.GenerateSet(party.ID(n))

	// structure holding parties' state and output
	states := map[party.ID]*state.State{}
	outputs := map[party.ID]*keygen.Output{}

	// create a state for each party
	for _, id := range partyIDs {
		states[id], outputs[id], err = frost.NewKeygenState(id, partyIDs, party.Size(t), 0)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	msgsOut1 := make([][]byte, 0, n)
	msgsOut2 := make([][]byte, 0, n*(n-1)/2)

	for _, s := range states {
		msgs1, err := helpers.PartyRoutine(nil, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut1 = append(msgsOut1, msgs1...)
	}

	for _, s := range states {
		msgs2, err := helpers.PartyRoutine(msgsOut1, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut2 = append(msgsOut2, msgs2...)
	}

	for _, s := range states {
		_, err := helpers.PartyRoutine(msgsOut2, s)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Get the public data
	fmt.Println("Group Key:")
	id0 := partyIDs[0]
	if err = states[id0].WaitForError(); err != nil {
		fmt.Println(err)
		return
	}
	public := outputs[id0].Public
	secrets := make(map[party.ID]*eddsa.SecretShare, n)
	groupKey := public.GroupKey
	fmt.Printf("  %x\n\n", groupKey.ToEd25519())

	for _, id := range partyIDs {
		if err := states[id].WaitForError(); err != nil {
			fmt.Println(err)
			return
		}
		shareSecret := outputs[id].SecretKey
		sharePublic := public.Shares[id]
		secrets[id] = shareSecret
		fmt.Printf("Party %d:\n  secret: %x\n  public: %x\n", id, shareSecret.Secret.Bytes(), sharePublic.Bytes())
	}

	// TODO: write JSON file, to take as input by CLI signer
	type KeyGenOutput struct {
		Secrets map[party.ID]*eddsa.SecretShare
		Shares  *eddsa.Public
	}

	for _, id := range partyIDs {

		// 创建一个新的 map 用于存储过滤后的 secretShare
		filteredSecrets := make(map[party.ID]*eddsa.SecretShare)

		// 遍历原始的 secrets map
		for nid, secret := range secrets {
			// 如果ID不等于'2'，则将其添加到新的 map 中
			if nid == id {
				filteredSecrets[id] = secret
			}
		}

		filteredShares := make(map[party.ID]*ristretto.Element)
		filteredShares[id] = public.Shares[id]

		filteredPubs := &eddsa.Public{
			partyIDs,
			party.Size(t),
			filteredShares,
			public.GroupKey,
		}

		kgOutput := KeyGenOutput{
			Secrets: filteredSecrets,
			Shares:  filteredPubs,
		}
		var jsonData []byte
		jsonData, err = json.MarshalIndent(kgOutput, "", " ")
		if err != nil {
			fmt.Println(err)
			return
		}

		filename := fmt.Sprintf("./keygenout_%d.json", id)

		_ = ioutil.WriteFile(filename, jsonData, 0644)

		fmt.Printf("Success: output written to %v\n", filename)

	}

	//kgOutput := KeyGenOutput{
	//	Secrets: secrets,
	//	Shares:  public,
	//}
	//
	//var jsonData []byte
	//jsonData, err = json.MarshalIndent(kgOutput, "", " ")
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//
	//filename := "./keygenout.json"
	//
	//_ = ioutil.WriteFile(filename, jsonData, 0644)
	//
	//fmt.Printf("Success: output written to %v\n", filename)
}

func verifyKeysV2(filename string, msg string) {

	message := []byte(msg)

	var err error

	type KeyGenOutput struct {
		Secrets map[party.ID]*eddsa.SecretShare
		Shares  *eddsa.Public
	}

	var kgOutput KeyGenOutput

	var jsonData []byte
	jsonData, err = ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = json.Unmarshal(jsonData, &kgOutput)
	if err != nil {
		fmt.Println(err)
		return
	}

	// get n and t from the keygen output
	var n party.Size
	var t party.Size

	n = kgOutput.Shares.PartyIDs.N()
	t = kgOutput.Shares.Threshold

	fmt.Printf("(t, n) = (%v, %v)\n", t, n)

	partyIDs := helpers.GenerateSet(n)

	secretShares := kgOutput.Secrets
	publicShares := kgOutput.Shares

	// structure holding parties' state and output
	states := map[party.ID]*state.State{}
	outputs := map[party.ID]*sign.Output{}

	msgsOut1 := make([][]byte, 0, n)
	msgsOut2 := make([][]byte, 0, n)

	for _, id := range partyIDs {

		states[id], outputs[id], err = frost.NewSignState(partyIDs, secretShares[id], publicShares, message, 0)
		if err != nil {
			fmt.Println()
		}
	}

	pk := publicShares.GroupKey

	for _, s := range states {
		msgs1, err := helpers.PartyRoutine(nil, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut1 = append(msgsOut1, msgs1...)
	}

	for _, s := range states {
		msgs2, err := helpers.PartyRoutine(msgsOut1, s)
		if err != nil {
			fmt.Println(err)
			return
		}
		msgsOut2 = append(msgsOut2, msgs2...)
	}

	for _, s := range states {
		_, err := helpers.PartyRoutine(msgsOut2, s)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	id0 := partyIDs[0]
	sig := outputs[id0].Signature
	if sig == nil {
		fmt.Println("null signature")
		return
	}

	if !ed25519.Verify(pk.ToEd25519(), message, sig.ToEd25519()) {
		fmt.Println("signature verification failed (ed25519)")
		return
	}

	if !pk.Verify(message, sig) {
		fmt.Println("signature verification failed")
		return
	}

	fmt.Printf("Success: signature is\nr: %x\ns: %x\n", sig.R.Bytes(), sig.S.Bytes())
}

func main() {
	//keygenDemo(2, 3)

	//verifyKeys("/Users/wuxi/code/mine/frost-ed25519/woods/keygenout3.json", "message_test111")
	//verifyKeysV2("/Users/wuxi/code/mine/frost-ed25519/woods/keygenout3.json", "message_test111")
	keygenDemoV2(2, 3)
}