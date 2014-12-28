package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/tendermint/tendermint/common"
)

//-----------------------------------------------------------------------------j
// Configuration types

type ConfigType struct {
	Network  string
	LAddr    string
	SeedNode string
	DB       DBConfig
	Alert    AlertConfig
	SMTP     SMTPConfig
	RPC      RPCConfig
}

type DBConfig struct {
	Backend string
	Dir     string
}

type AlertConfig struct {
	MinInterval int

	TwilioSid   string
	TwilioToken string
	TwilioFrom  string
	TwilioTo    string

	EmailRecipients []string
}

type SMTPConfig struct {
	User     string
	Password string
	Host     string
	Port     uint
}

type RPCConfig struct {
	HTTPPort uint
}

func (cfg *ConfigType) validate() error {
	if cfg.Network == "" {
		cfg.Network = defaultConfig.Network
	}
	if cfg.LAddr == "" {
		cfg.LAddr = defaultConfig.LAddr
	}
	if cfg.SeedNode == "" {
		cfg.SeedNode = defaultConfig.SeedNode
	}
	if cfg.DB.Backend == "" {
		return errors.New("DB.Backend must be set")
	}
	return nil
}

func (cfg *ConfigType) bytes() []byte {
	configBytes, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		panic(err)
	}
	return configBytes
}

func (cfg *ConfigType) write(configFile string) {
	if strings.Index(configFile, "/") != -1 {
		err := os.MkdirAll(filepath.Dir(configFile), 0700)
		if err != nil {
			panic(err)
		}
	}
	err := ioutil.WriteFile(configFile, cfg.bytes(), 0600)
	if err != nil {
		panic(err)
	}
}

//-----------------------------------------------------------------------------

var rootDir string
var defaultConfig ConfigType

func init() {
	// Get RootDir
	rootDir = os.Getenv("TMROOT")
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.tendermint"
	}

	// Compute defaultConfig
	defaultConfig = ConfigType{
		Network:  "tendermint_testnet0",
		LAddr:    "0.0.0.0:0",
		SeedNode: "",
		DB: DBConfig{
			Backend: "leveldb",
			Dir:     DataDir(),
		},
		Alert: AlertConfig{},
		SMTP:  SMTPConfig{},
		RPC: RPCConfig{
			HTTPPort: 8888,
		},
	}
}

func ConfigFile() string        { return rootDir + "/config.json" }
func GenesisFile() string       { return rootDir + "/genesis.json" }
func AddrBookFile() string      { return rootDir + "/addrbook.json" }
func PrivValidatorFile() string { return rootDir + "/priv_validator.json" }
func DataDir() string           { return rootDir + "/data" }

var Config ConfigType

func setFlags(printHelp *bool) {
	flag.BoolVar(printHelp, "help", false, "Print this help message.")
	flag.StringVar(&Config.LAddr, "laddr", Config.LAddr, "Listen address. (0.0.0.0:0 means any interface, any port)")
	flag.StringVar(&Config.SeedNode, "seed", Config.SeedNode, "Address of seed node")
}

func ParseFlags() {
	configFile := ConfigFile()

	// try to read configuration. if missing, write default
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		defaultConfig.write(configFile)
		fmt.Println("Config file written to config.json. Please edit & run again")
		os.Exit(1)
		return
	}

	// try to parse configuration. on error, die
	Config = ConfigType{}
	err = json.Unmarshal(configBytes, &Config)
	if err != nil {
		Exitf("Invalid configuration file %s: %v", configFile, err)
	}
	err = Config.validate()
	if err != nil {
		Exitf("Invalid configuration file %s: %v", configFile, err)
	}

	// try to parse arg flags, which can override file configuration.
	var printHelp bool
	setFlags(&printHelp)
	flag.Parse()
	if printHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}
}
