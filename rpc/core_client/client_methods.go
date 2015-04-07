package core_client

import (
	"fmt"
	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/rpc"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"io/ioutil"
	"net/http"
)

type Client interface {
	BlockchainInfo(minHeight uint) (*ctypes.ResponseBlockchainInfo, error)
	BroadcastTx(tx types.Tx) (*ctypes.ResponseBroadcastTx, error)
	Call(address []byte) (*ctypes.ResponseCall, error)
	DumpStorage(addr []byte) (*ctypes.ResponseDumpStorage, error)
	GenPrivAccount() (*ctypes.ResponseGenPrivAccount, error)
	GetAccount(address []byte) (*ctypes.ResponseGetAccount, error)
	GetBlock(height uint) (*ctypes.ResponseGetBlock, error)
	GetStorage(address []byte) (*ctypes.ResponseGetStorage, error)
	ListAccounts() (*ctypes.ResponseListAccounts, error)
	ListValidators() (*ctypes.ResponseListValidators, error)
	NetInfo() (*ctypes.ResponseNetInfo, error)
	SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*ctypes.ResponseSignTx, error)
	Status() (*ctypes.ResponseStatus, error)
}

func (c *ClientHTTP) BlockchainInfo(minHeight uint) (*ctypes.ResponseBlockchainInfo, error) {
	values, err := argsToURLValues([]string{"minHeight"}, minHeight)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"blockchain_info", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseBlockchainInfo `json:"result"`
		Error   string                         `json:"error"`
		Id      string                         `json:"id"`
		JSONRPC string                         `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) BroadcastTx(tx types.Tx) (*ctypes.ResponseBroadcastTx, error) {
	values, err := argsToURLValues([]string{"tx"}, tx)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"broadcast_tx", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseBroadcastTx `json:"result"`
		Error   string                      `json:"error"`
		Id      string                      `json:"id"`
		JSONRPC string                      `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) Call(address []byte) (*ctypes.ResponseCall, error) {
	values, err := argsToURLValues([]string{"address"}, address)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"call", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseCall `json:"result"`
		Error   string               `json:"error"`
		Id      string               `json:"id"`
		JSONRPC string               `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) DumpStorage(addr []byte) (*ctypes.ResponseDumpStorage, error) {
	values, err := argsToURLValues([]string{"addr"}, addr)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"dump_storage", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseDumpStorage `json:"result"`
		Error   string                      `json:"error"`
		Id      string                      `json:"id"`
		JSONRPC string                      `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GenPrivAccount() (*ctypes.ResponseGenPrivAccount, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"gen_priv_account", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGenPrivAccount `json:"result"`
		Error   string                         `json:"error"`
		Id      string                         `json:"id"`
		JSONRPC string                         `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GetAccount(address []byte) (*ctypes.ResponseGetAccount, error) {
	values, err := argsToURLValues([]string{"address"}, address)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"get_account", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGetAccount `json:"result"`
		Error   string                     `json:"error"`
		Id      string                     `json:"id"`
		JSONRPC string                     `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GetBlock(height uint) (*ctypes.ResponseGetBlock, error) {
	values, err := argsToURLValues([]string{"height"}, height)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"get_block", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGetBlock `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GetStorage(address []byte) (*ctypes.ResponseGetStorage, error) {
	values, err := argsToURLValues([]string{"address"}, address)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"get_storage", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGetStorage `json:"result"`
		Error   string                     `json:"error"`
		Id      string                     `json:"id"`
		JSONRPC string                     `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) ListAccounts() (*ctypes.ResponseListAccounts, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"list_accounts", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseListAccounts `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) ListValidators() (*ctypes.ResponseListValidators, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"list_validators", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseListValidators `json:"result"`
		Error   string                         `json:"error"`
		Id      string                         `json:"id"`
		JSONRPC string                         `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) NetInfo() (*ctypes.ResponseNetInfo, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"net_info", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseNetInfo `json:"result"`
		Error   string                  `json:"error"`
		Id      string                  `json:"id"`
		JSONRPC string                  `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*ctypes.ResponseSignTx, error) {
	values, err := argsToURLValues([]string{"tx", "privAccounts"}, tx, privAccounts)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"sign_tx", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseSignTx `json:"result"`
		Error   string                 `json:"error"`
		Id      string                 `json:"id"`
		JSONRPC string                 `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) Status() (*ctypes.ResponseStatus, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"status", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseStatus `json:"result"`
		Error   string                 `json:"error"`
		Id      string                 `json:"id"`
		JSONRPC string                 `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) BlockchainInfo(minHeight uint) (*ctypes.ResponseBlockchainInfo, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "blockchain_info",
		Params:  []interface{}{minHeight},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseBlockchainInfo `json:"result"`
		Error   string                         `json:"error"`
		Id      string                         `json:"id"`
		JSONRPC string                         `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) BroadcastTx(tx types.Tx) (*ctypes.ResponseBroadcastTx, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "broadcast_tx",
		Params:  []interface{}{tx},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseBroadcastTx `json:"result"`
		Error   string                      `json:"error"`
		Id      string                      `json:"id"`
		JSONRPC string                      `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) Call(address []byte) (*ctypes.ResponseCall, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "call",
		Params:  []interface{}{address},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseCall `json:"result"`
		Error   string               `json:"error"`
		Id      string               `json:"id"`
		JSONRPC string               `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) DumpStorage(addr []byte) (*ctypes.ResponseDumpStorage, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "dump_storage",
		Params:  []interface{}{addr},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseDumpStorage `json:"result"`
		Error   string                      `json:"error"`
		Id      string                      `json:"id"`
		JSONRPC string                      `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GenPrivAccount() (*ctypes.ResponseGenPrivAccount, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "gen_priv_account",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGenPrivAccount `json:"result"`
		Error   string                         `json:"error"`
		Id      string                         `json:"id"`
		JSONRPC string                         `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GetAccount(address []byte) (*ctypes.ResponseGetAccount, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "get_account",
		Params:  []interface{}{address},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGetAccount `json:"result"`
		Error   string                     `json:"error"`
		Id      string                     `json:"id"`
		JSONRPC string                     `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GetBlock(height uint) (*ctypes.ResponseGetBlock, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "get_block",
		Params:  []interface{}{height},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGetBlock `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GetStorage(address []byte) (*ctypes.ResponseGetStorage, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "get_storage",
		Params:  []interface{}{address},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseGetStorage `json:"result"`
		Error   string                     `json:"error"`
		Id      string                     `json:"id"`
		JSONRPC string                     `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) ListAccounts() (*ctypes.ResponseListAccounts, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "list_accounts",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseListAccounts `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) ListValidators() (*ctypes.ResponseListValidators, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "list_validators",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseListValidators `json:"result"`
		Error   string                         `json:"error"`
		Id      string                         `json:"id"`
		JSONRPC string                         `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) NetInfo() (*ctypes.ResponseNetInfo, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "net_info",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseNetInfo `json:"result"`
		Error   string                  `json:"error"`
		Id      string                  `json:"id"`
		JSONRPC string                  `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*ctypes.ResponseSignTx, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "sign_tx",
		Params:  []interface{}{tx, privAccounts},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseSignTx `json:"result"`
		Error   string                 `json:"error"`
		Id      string                 `json:"id"`
		JSONRPC string                 `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) Status() (*ctypes.ResponseStatus, error) {
	request := rpc.RPCRequest{
		JSONRPC: "2.0",
		Method:  "status",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *ctypes.ResponseStatus `json:"result"`
		Error   string                 `json:"error"`
		Id      string                 `json:"id"`
		JSONRPC string                 `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}
