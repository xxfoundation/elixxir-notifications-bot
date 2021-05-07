////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger

package cmd

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/notifications"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/utils"
	"net"
	"os"
	"path"
	"sync/atomic"
	"time"
)

var (
	cfgFile            string
	verbose            bool
	noTLS              bool
	NotificationParams notifications.Params
	loopDelay          int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "registration",
	Short: "Runs a registration server for cMix",
	Long:  `This server provides registration functions on cMix`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
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

		// Parse config file options
		certPath := viper.GetString("certPath")
		keyPath := viper.GetString("keyPath")
		localAddress := fmt.Sprintf("0.0.0.0:%d", viper.GetInt("port"))
		fbCreds, err := utils.ExpandPath(viper.GetString("firebaseCredentialsPath"))
		if err != nil {
			jww.FATAL.Panicf("Unable to expand credentials path: %+v", err)
		}

		// Populate params
		NotificationParams = notifications.Params{
			Address:  localAddress,
			CertPath: certPath,
			KeyPath:  keyPath,
			FBCreds:  fbCreds,
		}

		// Start notifications server
		jww.INFO.Println("Starting Notifications...")
		impl, err := notifications.StartNotifications(NotificationParams, noTLS, false)
		if err != nil {
			jww.FATAL.Panicf("Failed to start notifications server: %+v", err)
		}

		rawAddr := viper.GetString("dbAddress")
		var addr, port string
		if rawAddr != "" {
			addr, port, err = net.SplitHostPort(rawAddr)
			if err != nil {
				jww.FATAL.Panicf("Unable to get database port from %s: %+v", rawAddr, err)
			}
		}
		// Initialize the storage backend
		impl.Storage, err = storage.NewStorage(
			viper.GetString("dbUsername"),
			viper.GetString("dbPassword"),
			viper.GetString("dbName"),
			addr,
			port,
		)

		// Read in permissioning certificate
		cert, err := utils.ReadFile(viper.GetString("permissioningCertPath"))
		if err != nil {
			jww.FATAL.Panicf("Could not read permissioning cert: %+v", err)
		}

		// Add host for permissioning server
		hostParams := connect.GetDefaultHostParams()
		hostParams.AuthEnabled = false
		_, err = impl.Comms.AddHost(&id.Permissioning, viper.GetString("permissioningAddress"), cert, hostParams)
		if err != nil {
			jww.FATAL.Panicf("Failed to Create permissioning host: %+v", err)
		}

		// Start ephemeral ID tracking
		errChan := make(chan error)
		impl.TrackNdf()
		for atomic.LoadUint32(impl.ReceivedNdf()) != 1 {
			time.Sleep(time.Second)
		}
		go impl.EphIdCreator()
		go impl.EphIdDeleter()

		// Wait forever to prevent process from ending
		err = <-errChan
		jww.FATAL.Panicf("Notifications loop error received: %+v", err)
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

	rootCmd.Flags().StringVarP(&cfgFile, "config", "c",
		"", "Sets a custom config file path")

	rootCmd.Flags().BoolVar(&noTLS, "noTLS", false,
		"Runs without TLS enabled")

	rootCmd.Flags().IntVarP(&loopDelay, "loopDelay", "", 500,
		"Set the delay between notification loops (in milliseconds)")

	// Bind config and command line flags of the same name
	err := viper.BindPFlag("verbose", rootCmd.Flags().Lookup("verbose"))
	handleBindingError(err, "verbose")
}

// Handle flag binding errors
func handleBindingError(err error, flag string) {
	if err != nil {
		jww.FATAL.Panicf("Error on binding flag \"%s\":%+v", flag, err)
	}
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

		cfgFile = home + "/.elixxir/notifications.yaml"

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
		viper.WatchConfig()
	}
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
		logFile, err := os.OpenFile(logPath,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644)
		if err != nil {
			jww.WARN.Println("Invalid or missing log path, default path used.")
		} else {
			jww.SetLogOutput(logFile)
		}
	}
}
