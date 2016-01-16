package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/tendermint/netmon/handlers"
	"github.com/tendermint/netmon/types"

	"github.com/codegangsta/cli"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	pcm "github.com/tendermint/go-process"
	"github.com/tendermint/go-rpc/server"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

func init() {

	config := tmcfg.GetConfig("")
	config.Set("log_level", "debug")
	cfg.ApplyConfig(config) // Notify modules of new config
}

func main() {
	app := cli.NewApp()
	app.Name = "netmon"
	app.Usage = "netmon [command] [args...]"
	app.Commands = []cli.Command{
		{
			Name:      "config",
			Usage:     "Create a config from a mintnet testnet",
			ArgsUsage: "[chainID] [prefix] [N]",
			Action: func(c *cli.Context) {
				cmdConfig(c)
			},
		},
		{
			Name:      "monitor",
			Usage:     "Monitor a chain",
			ArgsUsage: "[config file]",
			Action: func(c *cli.Context) {
				cmdMonitor(c)
			},
		},
	}
	app.Run(os.Args)
}

func cmdMonitor(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		Exit("monitor expectes 1 arg")
	}
	chainConfigFile := args[0]

	chainConfig, err := types.LoadChainFromFile(chainConfigFile)
	if err != nil {
		Exit(err.Error())
	}

	// the main object that watches for changes and serves the rpc requests
	network := handlers.NewTendermintNetwork()
	network.RegisterChain(chainConfig)

	// the routes are functions on the network object
	routes := handlers.Routes(network)

	// serve http and ws
	mux := http.NewServeMux()
	wm := rpcserver.NewWebsocketManager(routes, nil) // TODO: evsw
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	rpcserver.RegisterRPCFuncs(mux, routes)
	if _, err := rpcserver.StartHTTPServer("0.0.0.0:46670", mux); err != nil {
		Exit(err.Error())
	}

	TrapSignal(func() {
		network.Stop()
	})

}

func cmdConfig(c *cli.Context) {
	args := c.Args()
	if len(args) != 3 {
		Exit("config expects 3 args")
	}
	id, prefix := args[0], args[1]
	n, err := strconv.Atoi(args[2])
	if err != nil {
		Exit(err.Error())
	}
	chain, err := ConfigFromMachines(id, prefix, n)
	if err != nil {
		Exit(err.Error())
	}

	b, err := json.Marshal(chain)
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println(string(b))
}

func ConfigFromMachines(chainID, prefix string, N int) (*types.BlockchainConfig, error) {

	chain := &types.BlockchainConfig{
		ID:         chainID,
		Validators: make([]*types.ChainValidator, N),
	}
	for i := 0; i < N; i++ {
		id := fmt.Sprintf("%s%d", prefix, i+1)
		ip, success := runProcessGetResult(id+"-ip", "docker-machine", []string{"ip", id})
		if !success {
			return nil, fmt.Errorf(ip)
		}

		val := &types.Validator{
			ID: id,
			// TODO: pubkey
		}
		chainVal := &types.ChainValidator{
			Validator: val,
			Addr:      fmt.Sprintf("%s:%d", strings.Trim(ip, "\n"), 46657),
			Index:     i,
		}
		chain.Validators[i] = chainVal
	}
	return chain, nil
}

func runProcessGetResult(label string, command string, args []string) (string, bool) {
	outFile := NewBufferCloser(nil)
	fmt.Println(Green(command), Green(args))
	proc, err := pcm.StartProcess(label, command, args, nil, outFile)
	if err != nil {
		return "", false
	}

	<-proc.WaitCh
	if proc.ExitState.Success() {
		fmt.Println(Blue(string(outFile.Bytes())))
		return string(outFile.Bytes()), true
	} else {
		// Error!
		fmt.Println(Red(string(outFile.Bytes())))
		return string(outFile.Bytes()), false
	}
}
