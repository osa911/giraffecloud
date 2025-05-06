package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
)

var logger *logging.Logger

func initLogger() {
	// Initialize logger configuration
	logConfig := &logging.LogConfig{
		File:       "~/.giraffecloud/client.log",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
	}

	// Initialize the global logger
	if err := logging.InitLogger(logConfig); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Get the logger instance
	logger = logging.GetGlobalLogger()
}

var rootCmd = &cobra.Command{
	Use:   "giraffecloud",
	Short: "GiraffeCloud CLI - Secure reverse tunnel client",
	Long: `GiraffeCloud CLI is a secure reverse tunnel client that allows you to expose
local services through GiraffeCloud's infrastructure.`,
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to GiraffeCloud and establish a tunnel",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Get tunnel host from flag if provided
		tunnelHost, _ := cmd.Flags().GetString("tunnel-host")
		if tunnelHost != "" {
			cfg.TunnelHost = tunnelHost
		}

		// Use TunnelHost for connection
		cfg.Server.Host = cfg.TunnelHost

		// Create TLS config
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.Security.InsecureSkipVerify,
		}

		// Load CA certificate if provided
		if cfg.Security.CACert != "" {
			caCert, err := os.ReadFile(cfg.Security.CACert)
			if err != nil {
				logger.Error("Failed to read CA certificate: %v", err)
				os.Exit(1)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				logger.Error("Failed to parse CA certificate")
				os.Exit(1)
			}

			tlsConfig.RootCAs = caCertPool
			logger.Info("Using custom CA certificate: %s", cfg.Security.CACert)
		}

		// Load client certificate if provided
		if cfg.Security.ClientCert != "" && cfg.Security.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(cfg.Security.ClientCert, cfg.Security.ClientKey)
			if err != nil {
				logger.Error("Failed to load client certificate: %v", err)
				os.Exit(1)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
			logger.Info("Using client certificate: %s", cfg.Security.ClientCert)
		}

		serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

		// Set up context and signal handling
		ctx, cancel := context.WithCancel(context.Background())
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigChan
			logger.Info("Received signal %v, initiating shutdown...", sig)
			cancel()
		}()

		retryDelay := 5 * time.Second
		maxDelay := 60 * time.Second
		attempt := 0

		logger.Info("Starting tunnel connection loop. Press Ctrl+C to stop.")
		for {
			select {
			case <-ctx.Done():
				logger.Info("Shutdown requested, exiting connect loop.")
				return
			default:
				t := tunnel.NewTunnel()
				logger.Info("Connecting to GiraffeCloud at %s (attempt %d)", serverAddr, attempt+1)

				// Spinner while connecting
				s := spinner.New(spinner.CharSets[14], 120*time.Millisecond)
				s.Suffix = " Connecting to GiraffeCloud..."
				s.Start()
				err := t.ConnectWithContext(ctx, serverAddr, cfg.Token, tlsConfig)
				s.Stop()

				if err != nil {
					if ctx.Err() != nil {
						logger.Info("Context canceled, exiting connect loop.")
						return
					}
					logger.Error("Failed to connect to GiraffeCloud: %v", err)
					attempt++
					delay := retryDelay * time.Duration(1<<uint(attempt))
					if delay > maxDelay {
						delay = maxDelay
					}
					logger.Info("Retrying in %s... (press Ctrl+C to exit)", delay)
					// Progress bar for reconnect delay
					bar := pb.New64(int64(delay.Seconds()))
					bar.SetTemplate(pb.Simple)
					bar.Set(pb.SIBytesPrefix, "")
					bar.Set(pb.Bytes, false)
					bar.Start()
					for i := 0; i < int(delay.Seconds()); i++ {
						if ctx.Err() != nil {
							bar.Finish()
							logger.Info("Context canceled during reconnect delay, exiting connect loop.")
							return
						}
						time.Sleep(1 * time.Second)
						bar.Increment()
					}
					bar.Finish()
					continue
				}
				logger.Info("Tunnel is running. Press Ctrl+C to stop.")
				// Wait for disconnect (keepAlive will exit on disconnect)
				for t.IsConnected() {
					if ctx.Err() != nil {
						logger.Info("Context canceled, disconnecting tunnel.")
						t.Disconnect()
						return
					}
					time.Sleep(1 * time.Second)
				}
				logger.Warn("Tunnel connection lost. Will attempt to reconnect.")
				attempt = 0 // Reset attempt counter after a successful connection
				// Short delay before reconnecting
				for i := 0; i < 2; i++ {
					if ctx.Err() != nil {
						logger.Info("Context canceled during reconnect wait, exiting connect loop.")
						return
					}
					time.Sleep(1 * time.Second)
				}
			}
		}
	},
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage GiraffeCloud service",
	Long:  `Install, uninstall, or manage the GiraffeCloud service on your system.`,
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install GiraffeCloud as a system service",
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := tunnel.NewServiceManager()
		if err != nil {
			logger.Error("Failed to create service manager: %v", err)
			os.Exit(1)
		}

		if err := sm.Install(); err != nil {
			logger.Error("Failed to install service: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully installed GiraffeCloud service")
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall GiraffeCloud system service",
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := tunnel.NewServiceManager()
		if err != nil {
			logger.Error("Failed to create service manager: %v", err)
			os.Exit(1)
		}

		if err := sm.Uninstall(); err != nil {
			logger.Error("Failed to uninstall service: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully uninstalled GiraffeCloud service")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("GiraffeCloud version: %s", version.Info())
	},
}

var loginCmd = &cobra.Command{
	Use:   "login --token <TOKEN>",
	Short: "Login to GiraffeCloud using an API token",
	Long: `Login to GiraffeCloud using an API token.
The token will be stored securely in your config file (~/.giraffecloud/config).
This command will also fetch your client certificates from the server.

After successful login, use 'giraffecloud connect' to establish a tunnel connection.

Example:
  1. giraffecloud login --token your-api-token
  2. giraffecloud connect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := cmd.Flags().GetString("token")
		if err != nil {
			logger.Error("Failed to get token flag: %v", err)
			return err
		}

		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Failed to load config: %v", err)
			return err
		}

		// Get API host from flag if provided
		apiHost, _ := cmd.Flags().GetString("api-host")
		if apiHost != "" {
			cfg.APIHost = apiHost
		}

		// Use APIHost for certificate fetching
		serverHost := cfg.APIHost

		// Create certificates directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory: %v", err)
			return err
		}
		certsDir := filepath.Join(homeDir, ".giraffecloud", "certs")
		if err := os.MkdirAll(certsDir, 0700); err != nil {
			logger.Error("Failed to create certificates directory: %v", err)
			return err
		}

		// Fetch certificates from server
		logger.Info("Fetching certificates from server...")
		s := spinner.New(spinner.CharSets[14], 120*time.Millisecond)
		s.Suffix = " Fetching certificates from server..."
		s.Writer = os.Stdout
		s.Start()
		err = tunnel.FetchCertificates(serverHost, token, certsDir)
		s.Stop()
		fmt.Println() // Ensure spinner and logs don't overlap
		if err != nil {
			logger.Error("Failed to fetch certificates: %v", err)
			return err
		}
		logger.Info("Successfully downloaded certificates")

		// Update config with certificate paths
		cfg.Token = token
		cfg.Security.CACert = filepath.Join(certsDir, "ca.crt")
		cfg.Security.ClientCert = filepath.Join(certsDir, "client.crt")
		cfg.Security.ClientKey = filepath.Join(certsDir, "client.key")

		// Save the updated config
		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			return err
		}

		logger.Info("Successfully logged in to GiraffeCloud (server: %s)", serverHost)
		logger.Info("Certificates stored in: %s", certsDir)
		logger.Info("Run 'giraffecloud connect' to establish a tunnel connection")
		return nil
	},
}

func init() {
	// Initialize logger first
	initLogger()
	logger.Info("Initializing GiraffeCloud CLI")

	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(loginCmd)

	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)

	// Add host flag to connect command
	connectCmd.Flags().String("tunnel-host", "", "Tunnel host to connect to (default: tunnel.giraffecloud.xyz)")

	// Add host flag to login command
	loginCmd.Flags().String("api-host", "", "API host for login/certificates (default: api.giraffecloud.xyz)")
	loginCmd.Flags().String("token", "", "API token for authentication")
	loginCmd.MarkFlagRequired("token")

	logger.Debug("CLI commands and flags initialized")
}

func main() {
	defer logger.Close()

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed: %v", err)
		os.Exit(1)
	}
}