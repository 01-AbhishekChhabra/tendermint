package main

import (
	"fmt"
	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/rpc"
)

// These are convenience functions for a single developer.
// When multiple are involved, the workflow is different.
// (First the command(s) are signed by all validators,
//  and then it is broadcast).

func RunProcess(privKey acm.PrivKey, remote string, command btypes.CommandRunProcess) (response btypes.ResponseRunProcess, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func StopProcess(privKey acm.PrivKey, remote string, command btypes.CommandStopProcess) (response btypes.ResponseStopProcess, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func ListProcesses(privKey acm.PrivKey, remote string, command btypes.CommandListProcesses) (response btypes.ResponseListProcesses, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

//-----------------------------------------------------------------------------

// Utility method to get nonce from the remote.
// The next command should include the returned nonce+1 as nonce.
func GetNonce(remote string) (uint64, error) {
	var err error
	response := btypes.ResponseStatus{}
	_, err = rpc.Call(remote, "status", Arr(), &response)
	if err != nil {
		return 0, fmt.Errorf("Error fetching nonce from remote %v:\n  %v", remote, err)
	}
	return response.Nonce, nil
}

// Each developer runs this
func SignCommand(privKey acm.PrivKey, nonce uint64, command btypes.Command) ([]byte, acm.Signature) {
	noncedCommand := btypes.NoncedCommand{
		Nonce:   nonce,
		Command: command,
	}
	commandJSONBytes := binary.JSONBytes(noncedCommand)
	signature := privKey.Sign(commandJSONBytes)
	return commandJSONBytes, signature
}

// Somebody aggregates the signatures and calls this.
func RunAuthCommand(remote string, commandJSONBytes []byte, signatures []acm.Signature, dest interface{}) (interface{}, error) {
	authCommand := btypes.AuthCommand{
		CommandJSONStr: string(commandJSONBytes),
		Signatures:     signatures,
	}
	return rpc.Call(remote, "run", Arr(authCommand), dest)
}
