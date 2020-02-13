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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/notifications-bot/notifications"
	"gitlab.com/elixxir/notifications-bot/storage"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
	"os"
	"path"
	"strings"
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

		// Initialize the storage backend
		impl.Storage = storage.NewDatabase(
			viper.GetString("dbUsername"),
			viper.GetString("dbPassword"),
			viper.GetString("dbName"),
			viper.GetString("dbAddress"),
		)

		// Set up the notifications server connections
		err = setupConnection(impl, viper.GetString("permissioningCertPath"), viper.GetString("permissioningAddress"))
		if err != nil {
			jww.FATAL.Panicf("Failed to set up connections: %+v", err)
		}

		// Start notification loop
		killChan := make(chan struct{})
		errChan := make(chan error)
		go impl.RunNotificationLoop(loopDelay, killChan, errChan)

		// Wait forever to prevent process from ending
		err = <-errChan
		panic(err)
	},
}

// setupConnection handles connecting to permissioning and polling for the NDF once connected
func setupConnection(impl *notifications.Impl, permissioningCertPath, permissioningAddr string) error {
	// Read in permissioning certificate
	cert, err := utils.ReadFile(permissioningCertPath)
	if err != nil {
		return errors.Wrap(err, "Could not read permissioning cert")
	}

	// Add host for permissioning server
	_, err = impl.Comms.AddHost(id.PERMISSIONING, permissioningAddr, cert, true, false)
	if err != nil {
		return errors.Wrap(err, "Failed to Create permissioning host")
	}

	// Loop until an NDF is received
	var def *ndf.NetworkDefinition
	emptyNdf := &ndf.NetworkDefinition{}
	for def == nil {
		def, err = impl.Comms.RetrieveNdf(emptyNdf)
		// Don't stop if error is expected
		if err != nil && !strings.Contains(err.Error(), ndf.NO_NDF) {
			return errors.Wrap(err, "Failed to get NDF")
		}
	}

	// Update NDF & gateway host
	err = impl.UpdateNdf(def)
	if err != nil {
		return errors.Wrap(err, "Failed to update impl's NDF")
	}
	return nil
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
		logFile, err := os.Create(logPath)
		if err != nil {
			jww.WARN.Println("Invalid or missing log path, default path used.")
		} else {
			jww.SetLogOutput(logFile)
		}
	}
}
