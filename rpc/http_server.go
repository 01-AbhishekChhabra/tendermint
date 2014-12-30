package rpc

import (
	"fmt"
	"net/http"

	. "github.com/tendermint/tendermint/config"
	. "github.com/tendermint/tendermint/common"
)

func StartHTTPServer() {

	http.HandleFunc("/block", BlockHandler)

	// Serve HTTP on localhost only.
	// Let something like Nginx handle HTTPS connections.
	address := fmt.Sprintf("127.0.0.1:%v", Config.RPC.HTTPPort)
	log.Info(Fmt("Starting RPC HTTP server on http://%s", address))

	go func() {
		log.Crit("%v", http.ListenAndServe(address, RecoverAndLogHandler(http.DefaultServeMux)))
	}()
}
