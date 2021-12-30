////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger

package cmd

import (
	"fmt"
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
	"sync/atomic"
	"time"
)

var (
	cfgFile, logPath   string
	verbose            bool
	noTLS              bool
	validConfig        bool
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
		initConfig()
		initLog()

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

		apnsKeyPath, err := utils.ExpandPath(viper.GetString("apnsKeyPath"))
		if err != nil {
			jww.FATAL.Panicf("Unable to expand apns key path: %+v", err)
		}
		viper.SetDefault("notificationRate", 30)
		viper.SetDefault("notificationsPerBatch", 20)
		viper.SetDefault("maxNotificationPayload", 4096)
		// Populate params
		NotificationParams = notifications.Params{
			Address:                localAddress,
			CertPath:               certPath,
			KeyPath:                keyPath,
			FBCreds:                fbCreds,
			NotificationRate:       viper.GetInt("notificationRate"),
			NotificationsPerBatch:  viper.GetInt("notificationsPerBatch"),
			MaxNotificationPayload: viper.GetInt("maxNotificationPayload"),
			APNS: notifications.APNSParams{
				KeyPath:  apnsKeyPath,
				KeyID:    viper.GetString("apnsKeyID"),
				Issuer:   viper.GetString("apnsIssuer"),
				BundleID: viper.GetString("apnsBundleID"),
				Dev:      viper.GetBool("apnsDev"),
			},
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
		s, err := storage.NewStorage(
			viper.GetString("dbUsername"),
			viper.GetString("dbPassword"),
			viper.GetString("dbName"),
			addr,
			port,
		)
		if err != nil {
			jww.FATAL.Panicf("Failed to initialize storage: %+v", err)
		}

		// Start notifications server
		jww.INFO.Println("Starting Notifications...")
		impl, err := notifications.StartNotifications(NotificationParams, noTLS, false)
		if err != nil {
			jww.FATAL.Panicf("Failed to start notifications server: %+v", err)
		}

		impl.Storage = s

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
	var err error
	validConfig = true
	if cfgFile == "" {
		cfgFile, err = utils.SearchDefaultLocations("notifications.yaml", "xxnetwork")
		if err != nil {
			validConfig = false
			jww.FATAL.Panicf("Failed to find config file: %+v", err)
		}
	} else {
		cfgFile, err = utils.ExpandPath(cfgFile)
		if err != nil {
			validConfig = false
			jww.FATAL.Panicf("Failed to expand config file path: %+v", err)
		}
	}

	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Unable to read config file (%s): %+v", cfgFile, err.Error())
		validConfig = false
	}
}

// initLog initializes logging thresholds and the log path.
func initLog() {
	vipLogLevel := viper.GetUint("logLevel")

	// Check the level of logs to display
	if vipLogLevel > 1 {
		// Set the GRPC log level
		err := os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
		if err != nil {
			jww.ERROR.Printf("Could not set GRPC_GO_LOG_SEVERITY_LEVEL: %+v", err)
		}

		err = os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
		if err != nil {
			jww.ERROR.Printf("Could not set GRPC_GO_LOG_VERBOSITY_LEVEL: %+v", err)
		}
		// Turn on trace logs
		jww.SetLogThreshold(jww.LevelTrace)
		jww.SetStdoutThreshold(jww.LevelTrace)
		mixmessages.TraceMode()
	} else if vipLogLevel == 1 {
		// Turn on debugging logs
		jww.SetLogThreshold(jww.LevelDebug)
		jww.SetStdoutThreshold(jww.LevelDebug)
		mixmessages.DebugMode()
	} else {
		// Turn on info logs
		jww.SetLogThreshold(jww.LevelInfo)
		jww.SetStdoutThreshold(jww.LevelInfo)
	}

	logPath = viper.GetString("log")

	logFile, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644)
	if err != nil {
		fmt.Printf("Could not open log file %s!\n", logPath)
	} else {
		jww.SetLogOutput(logFile)
	}
}
