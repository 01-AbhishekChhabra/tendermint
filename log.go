package main

import (
	"github.com/cihub/seelog"
	"github.com/tendermint/tendermint/p2p"
)

var log seelog.LoggerInterface

func init() {
	// TODO: replace with configuration file in the ~/.tendermint directory.
	config := `
<seelog type="sync" minlevel="debug">
    <outputs formatid="colored">
        <console/>
    </outputs>
    <formats>
        <format id="main"       format="%Date/%Time [%LEV] %Msg%n"/>
        <format id="colored"    format="%Time %EscM(46)%Level%EscM(49) %EscM(36)%File%EscM(39) %Msg%n%EscM(0)"/>
    </formats>
</seelog>`

	var err error
	log, err = seelog.LoggerFromConfigAsBytes([]byte(config))
	if err != nil {
		panic(err)
	}

	p2p.SetLogger(log)
}
