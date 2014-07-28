package main

import (
	"os"

	"github.com/op/go-logging"
	"github.com/tendermint/tendermint/block"
	"github.com/tendermint/tendermint/p2p"
)

var log = logging.MustGetLogger("main")

func init() {
	// Customize the output format
	logging.SetFormatter(logging.MustStringFormatter("[%{level:.4s}] %{time:2006-01-02T15:04:05} %{shortfile:-20s} %{message}"))

	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logBackend.Color = true
	logging.SetBackend(logBackend)

	// Test
	/*
	   Log.Debug("debug")
	   Log.Info("info")
	   Log.Notice("notice")
	   Log.Warning("warning")
	   Log.Error("error")
	*/

	p2p.SetLogger(log)
	block.SetLogger(log)
}
