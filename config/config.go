package config

import (
	"log"
	"os"
	"os/user"
	"strings"
	"sync"

	"gopkg.in/gcfg.v1"
)

var cfg *AnchorConfig

type AnchorConfig struct {
	App struct {
		HomeDir                       string
		DBType                        string
		LdbPath                       string
		BoltPath                      string
		FactomdNodeURL                string
		ECPrivateKey                  string
		AllAnchorRecordPublicKeys     []string
		CurrentAnchorRecordPrivateKey string
		WindowSize                    uint32
	}
	Ethereum struct {
		WalletAddress      string
		WalletKeyPath      string
		WalletPassword     string
		ContractAddress    string
		GasLimit           string
		GethNodeURL        string
		GethIPCURL         string
		EthGasStationURL   string
		IgnoreWrongEntries bool
		TestNet            bool
		TestNetName        string
	}
	Log struct {
		LogPath  string
		LogLevel string
	}

	Proxy          string `long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	DisableListen  bool   `long:"nolisten" description:"Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen"`
	DisableRPC     bool   `long:"norpc" description:"Disable built-in RPC server -- NOTE: The RPC server is disabled by default if no rpcuser/rpcpass is specified"`
	DisableTLS     bool   `long:"notls" description:"Disable TLS for the RPC server -- NOTE: This is only allowed if the RPC server is bound to localhost"`
	DisableDNSSeed bool   `long:"nodnsseed" description:"Disable DNS seeding for peers"`
}

const defaultConfig = `
; ------------------------------------------------------------------------------
; App settings
; ------------------------------------------------------------------------------
[app]
HomeDir                             = ""
; --------------- DBType: LDB | Bolt | Map
DBType                              = "Map"
LdbPath                             = "AnchormakerLDB"
BoltPath                            = "AnchormakerBolt.db"
FactomdNodeURL                      = "localhost:8088"
ECPrivateKey                        = "Es2Rf7iM6PdsqfYCo3D1tnAR65SkLENyWJG1deUzpRMQmbh9F3eG" ; all zeros. public key = EC2DKSYyRcNWf7RS963VFYgMExoHRYLHVeCfQ9PGPmNzwrcmgm2r
AllAnchorRecordPublicKeys           = "3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29"
CurrentAnchorRecordPrivateKey       = "0000000000000000000000000000000000000000000000000000000000000000" ; public key = 3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29
WindowSize                          = 1000

; ------------------------------------------------------------------------------
; Ethereum settings
; ------------------------------------------------------------------------------
[ethereum]
WalletAddress                       = "0x84964e1FfC60d0ad4DA803678b167c6A783A2E01"
WalletKeyPath                       = ""
WalletPassword                      = "password"
ContractAddress                     = "0xd1932fe27273e0dc1a2fa5257c75811fd5555a1d"
GasLimit                            = "200000"
GethNodeURL                         = "localhost:8545"
GethIPCURL                          = "/home/sam/.ethereum/testnet/geth.ipc"
EthGasStationURL                    = "https://ethgasstation.info/json/ethgasAPI.json"
IgnoreWrongEntries                  = true
TestNet                             = true
TestNetName                         = "ropsten"

; ------------------------------------------------------------------------------
; logLevel - allowed values are: debug, info, notice, warning, error, critical, alert, emergency and none
; ------------------------------------------------------------------------------
[log]
logLevel                            = debug
LogPath                             = "anchormaker.log"
`

var once sync.Once
var filename = getHomeDir() + "/.factom/anchormaker.conf"

func SetConfigFile(f string) {
	filename = f
}

// GetConfig reads the default anchormaker.conf file and returns an AnchorConfig
// object corresponding to the state of the file.
func ReadConfig() *AnchorConfig {
	once.Do(func() {
		cfg = readAnchorConfig()
	})
	return cfg
}

func ReReadConfig() *AnchorConfig {
	return readAnchorConfig()
}

func readAnchorConfig() *AnchorConfig {
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}
	if strings.HasPrefix(filename, "~") {
		filename = getHomeDir() + filename
	}
	cfg := new(AnchorConfig)
	//log.Println("read anchormaker config file: ", filename)

	err := gcfg.ReadFileInto(cfg, filename)
	if err != nil {
		log.Println("ERROR Reading config file!\nServer starting with default settings...\n", err)
		gcfg.ReadStringInto(cfg, defaultConfig)
	}

	// Default to home directory if not set
	if len(cfg.App.HomeDir) < 1 {
		cfg.App.HomeDir = getHomeDir() + "/.factom/"
	}

	// TODO: improve the paths after milestone 1
	cfg.App.LdbPath = cfg.App.HomeDir + cfg.App.LdbPath
	cfg.App.BoltPath = cfg.App.HomeDir + cfg.App.BoltPath
	cfg.Log.LogPath = cfg.App.HomeDir + cfg.Log.LogPath

	return cfg
}

func getHomeDir() string {
	// Get the OS specific home directory via the Go standard lib.
	var homeDir string
	usr, err := user.Current()
	if err == nil {
		homeDir = usr.HomeDir
	}

	// Fall back to standard HOME environment variable that works
	// for most POSIX OSes if the directory from the Go standard
	// lib failed.
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	return homeDir
}
