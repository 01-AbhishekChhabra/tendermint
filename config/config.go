package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	//"crypto/rand"
	//"encoding/hex"
)

/* Global & initialization */

var AppDir = os.Getenv("HOME") + "/.tendermint"
var Config Config_

func initFlags(printHelp *bool) {
	flag.BoolVar(printHelp, "help", false, "Print this help message.")
	flag.StringVar(&Config.IP, "ip", Config.IP, "Listen IP. (0.0.0.0 means any)")
	flag.IntVar(&Config.Port, "port", Config.Port, "Listen port. (0 means any)")
	flag.StringVar(&Config.Seed, "seed", Config.Seed, "Address of seed node")
}

func init() {
	configFile := AppDir + "/config.json"

	// try to read configuration. if missing, write default
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		defaultConfig.write(configFile)
		fmt.Println("Config file written to config.json. Please edit & run again")
		os.Exit(1)
		return
	}

	// try to parse configuration. on error, die
	Config = Config_{}
	err = json.Unmarshal(configBytes, &Config)
	if err != nil {
		log.Panicf("Invalid configuration file %s: %v", configFile, err)
	}
	err = Config.validate()
	if err != nil {
		log.Panicf("Invalid configuration file %s: %v", configFile, err)
	}

	// try to parse arg flags, which can override file configuration.
	var printHelp bool
	initFlags(&printHelp)
	flag.Parse()
	if printHelp {
		fmt.Println("----------------------------------")
		flag.PrintDefaults()
		fmt.Println("----------------------------------")
		os.Exit(0)
	}
}

/* Default configuration */

var defaultConfig = Config_{
	IP:   "0.0.0.0",
	Port: 8770,
	Seed: "",
	Db: DbConfig{
		Type: "level",
		Dir:  AppDir + "/data",
	},
	Twilio: TwilioConfig{},
}

/* Configuration types */

type Config_ struct {
	IP     string
	Port   int
	Seed   string
	Db     DbConfig
	Twilio TwilioConfig
}

type TwilioConfig struct {
	Sid         string
	Token       string
	From        string
	To          string
	MinInterval int
}

type DbConfig struct {
	Type string
	Dir  string
}

func (cfg *Config_) validate() error {
	if cfg.IP == "" {
		return errors.New("IP must be set")
	}
	if cfg.Port == 0 {
		return errors.New("Port must be set")
	}
	if cfg.Db.Type == "" {
		return errors.New("Db.Type must be set")
	}
	return nil
}

func (cfg *Config_) bytes() []byte {
	configBytes, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		panic(err)
	}
	return configBytes
}

func (cfg *Config_) write(configFile string) {
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

/* TODO: generate priv/pub keys
func generateKeys() string {
    bytes := &[30]byte{}
    rand.Read(bytes[:])
    return hex.EncodeToString(bytes[:])
}
*/
