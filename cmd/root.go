////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger

package cmd

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/registration/database"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

var (
	cfgFile              string
	verbose              bool
	showVer              bool
	noTLS                bool
	RegistrationCodes    []string
	RegParams            Params
	ClientRegCodes       []string
	udbParams            ndf.UDB
	clientVersion        string
	clientVersionLock    sync.RWMutex
	disablePermissioning bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "registration",
	Short: "Runs a registration server for cMix",
	Long:  `This server provides registration functions on cMix`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if showVer {
			printVersion()
			return
		}

		if verbose {
			err := os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
			if err != nil {
				jww.ERROR.Printf("Could not set GRPC_GO_LOG_SEVERITY_LEVEL: %+v", err)
			}

			err = os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "2")
			if err != nil {
				jww.ERROR.Printf("Could not set GRPC_GO_LOG_VERBOSITY_LEVEL: %+v", err)
			}
		}

		cmixMap := viper.GetStringMapString("groups.cmix")
		e2eMap := viper.GetStringMapString("groups.e2e")

		cmix := toGroup(cmixMap)
		e2e := toGroup(e2eMap)

		// Parse config file options
		certPath := viper.GetString("certPath")
		keyPath := viper.GetString("keyPath")
		localAddress := fmt.Sprintf("0.0.0.0:%d", viper.GetInt("port"))
		ndfOutputPath := viper.GetString("ndfOutputPath")
		setClientVersion(viper.GetString("clientVersion"))
		ipAddr := viper.GetString("publicAddress")

		publicAddress := fmt.Sprintf("%s:%d", ipAddr, viper.GetInt("port"))

		// Set up database connection
		database.PermissioningDb = database.NewDatabase(
			viper.GetString("dbUsername"),
			viper.GetString("dbPassword"),
			viper.GetString("dbName"),
			viper.GetString("dbAddress"),
		)

		// Populate Node registration codes into the database
		RegistrationCodes = viper.GetStringSlice("registrationCodes")
		database.PopulateNodeRegistrationCodes(RegistrationCodes)

		ClientRegCodes = viper.GetStringSlice("clientRegCodes")
		database.PopulateClientRegistrationCodes(ClientRegCodes, 1000)

		//Fixme: Do we want the udbID to be specified in the yaml?
		tmpSlice := make([]byte, 32)

		tmpSlice[len(tmpSlice)-1] = byte(viper.GetInt("udbID"))
		udbParams.ID = tmpSlice
		// Populate params
		RegParams = Params{
			Address:       localAddress,
			CertPath:      certPath,
			KeyPath:       keyPath,
			NdfOutputPath: ndfOutputPath,
			cmix:          cmix,
			e2e:           e2e,
			publicAddress: publicAddress,
		}
		jww.INFO.Println("Starting Permissioning")
		jww.INFO.Println("Starting User Registration")
		// Start registration server
		impl := StartRegistration(RegParams)

		// Begin the thread which handles the completion of node registration
		go nodeRegistrationCompleter(impl)

		// Wait forever to prevent process from ending
		select {}

	},
}

// Execute adds all child commands to the root command and sets flags
// appropriately.  This is called by main.main(). It only needs to
// happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		jww.ERROR.Println(err)
		os.Exit(1)
	}
}

// init is the initialization function for Cobra which defines commands
// and flags.
func init() {
	// NOTE: The point of init() is to be declarative.
	// There is one init in each sub command. Do not put variable declarations
	// here, and ensure all the Flags are of the *P variety, unless there's a
	// very good reason not to have them as local params to sub command."
	cobra.OnInitialize(initConfig, initLog)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Show verbose logs for debugging")
	rootCmd.Flags().BoolVarP(&showVer, "version", "V", false,
		"Show version information")

	rootCmd.Flags().StringVarP(&cfgFile, "config", "c",
		"", "Sets a custom config file path")

	rootCmd.Flags().BoolVar(&noTLS, "noTLS", false,
		"Runs without TLS enabled")

	rootCmd.Flags().BoolVarP(&disablePermissioning, "disablePermissioning", "",
		false, "Disables registration server checking for ndf updates")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	//Use default config location if none is passed
	if cfgFile == "" {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			jww.ERROR.Println(err)
			os.Exit(1)
		}

		cfgFile = home + "/.elixxir/registration.yaml"

	}

	validConfig := true
	f, err := os.Open(cfgFile)
	if err != nil {
		jww.ERROR.Printf("Unable to open config file (%s): %+v", cfgFile, err)
		validConfig = false
	}
	_, err = f.Stat()
	if err != nil {
		jww.ERROR.Printf("Invalid config file (%s): %+v", cfgFile, err)
		validConfig = false
	}
	err = f.Close()
	if err != nil {
		jww.ERROR.Printf("Unable to close config file (%s): %+v", cfgFile, err)
		validConfig = false
	}

	// Set the config file if it is valid
	if validConfig {
		// Set the config path to the directory containing the config file
		// This may increase the reliability of the config watching, somewhat
		cfgDir, _ := path.Split(cfgFile)
		viper.AddConfigPath(cfgDir)

		viper.SetConfigFile(cfgFile)
		viper.AutomaticEnv() // read in environment variables that match

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err != nil {
			jww.ERROR.Printf("Unable to parse config file (%s): %+v", cfgFile, err)
			validConfig = false
		}
		viper.OnConfigChange(updateClientVersion)
		viper.WatchConfig()
	}
}

func updateClientVersion(in fsnotify.Event) {
	newVersion := viper.GetString("clientVersion")
	err := validateVersion(newVersion)
	if err != nil {
		panic(err)
	}
	setClientVersion(newVersion)
}

func setClientVersion(version string) {
	clientVersionLock.Lock()
	clientVersion = version
	clientVersionLock.Unlock()
}

func validateVersion(versionString string) error {
	// If a version string has more than 2 dots in it, anything after the first
	// 2 dots is considered to be part of the patch version
	versions := strings.SplitN(versionString, ".", 3)
	if len(versions) != 3 {
		return errors.New("Client version string must contain a major, minor, and patch version separated by \".\"")
	}
	_, err := strconv.Atoi(versions[0])
	if err != nil {
		return errors.New("Major client version couldn't be parsed as integer")
	}
	_, err = strconv.Atoi(versions[1])
	if err != nil {
		return errors.New("Minor client version couldn't be parsed as integer")
	}
	return nil
}

// initLog initializes logging thresholds and the log path.
func initLog() {
	if viper.Get("logPath") != nil {
		// If verbose flag set then log more info for debugging
		if verbose || viper.GetBool("verbose") {
			jww.SetLogThreshold(jww.LevelDebug)
			jww.SetStdoutThreshold(jww.LevelDebug)
			mixmessages.DebugMode()
		} else {
			jww.SetLogThreshold(jww.LevelInfo)
			jww.SetStdoutThreshold(jww.LevelInfo)
		}
		// Create log file, overwrites if existing
		logPath := viper.GetString("logPath")
		logFile, err := os.Create(logPath)
		if err != nil {
			jww.WARN.Println("Invalid or missing log path, default path used.")
		} else {
			jww.SetLogOutput(logFile)
		}
	}
}
