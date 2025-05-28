package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
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
	Long: `Connect to GiraffeCloud and establish a tunnel to expose your local service.
The tunnel will forward requests from your assigned domain to your local service.

Example:
  giraffecloud connect                  # Use port configured on server
  giraffecloud connect --local-port 3000  # Override with local port 3000`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Get tunnel host and port from flags if provided
		tunnelHost, _ := cmd.Flags().GetString("tunnel-host")
		tunnelPort, _ := cmd.Flags().GetInt("tunnel-port")
		localPort, _ := cmd.Flags().GetInt("local-port")
		if tunnelHost != "" {
			cfg.Server.Host = tunnelHost
		}
		if tunnelPort != 0 {
			cfg.Server.Port = tunnelPort
		}
		if localPort != 0 {
			cfg.LocalPort = localPort
		}

		serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

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

		// Set up context and signal handling
		ctx, cancel := context.WithCancel(context.Background())
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigChan
			logger.Info("Received signal %v, initiating shutdown...", sig)
			cancel()
		}()

		logger.Info("Starting tunnel connection to %s", serverAddr)
		if cfg.LocalPort > 0 {
			logger.Info("Will forward requests to localhost:%d (overridden by --local-port flag)", cfg.LocalPort)
		} else {
			logger.Info("Will use port from server configuration")
		}

		t := tunnel.NewTunnel()

		// Spinner while connecting
		s := spinner.New(spinner.CharSets[14], 120*time.Millisecond)
		s.Suffix = " Connecting to GiraffeCloud..."
		s.Start()
		err = t.Connect(serverAddr, cfg.Token, cfg.Domain, cfg.LocalPort, tlsConfig)
		s.Stop()

		if err != nil {
			logger.Error("Failed to connect to GiraffeCloud: %v", err)
			os.Exit(1)
		}

		logger.Info("Tunnel is running. Press Ctrl+C to stop.")

		<-ctx.Done()
		logger.Info("Shutting down tunnel...")
		t.Disconnect()
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
	Use:   "login",
	Short: "Login to GiraffeCloud and obtain certificates",
	Long: `Login to GiraffeCloud using an API token.
The token will be stored securely in your config file (~/.giraffecloud/config).
This command will also fetch your client certificates from the server.

After successful login, use 'giraffecloud connect' to establish a tunnel connection.

Example:
  giraffecloud login --token your-api-token [--api-host api.giraffecloud.xyz] [--api-port 443]`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Get API host and port from flags if provided
		apiHost, _ := cmd.Flags().GetString("api-host")
		apiPort, _ := cmd.Flags().GetInt("api-port")
		if apiHost != "" {
			cfg.API.Host = apiHost
		}
		if apiPort != 0 {
			cfg.API.Port = apiPort
		}

		token, _ := cmd.Flags().GetString("token")
		cfg.Token = token

		// Create certificates directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory: %v", err)
			os.Exit(1)
		}
		certsDir := filepath.Join(homeDir, ".giraffecloud", "certs")
		if err := os.MkdirAll(certsDir, 0700); err != nil {
			logger.Error("Failed to create certificates directory: %v", err)
			os.Exit(1)
		}

		// Fetch certificates from API server
		certResp, err := handlers.FetchCertificates(cfg.API.Host, cfg.API.Port, cfg.Token)
		if err != nil {
			logger.Error("Failed to fetch certificates: %v", err)
			os.Exit(1)
		}
		logger.Info("Successfully downloaded certificates")

		// Save certificates to files
		files := map[string]string{
			"ca.crt":     certResp.CACert,
			"client.crt": certResp.ClientCert,
			"client.key": certResp.ClientKey,
		}

		for filename, content := range files {
			path := filepath.Join(certsDir, filename)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				logger.Error("Failed to write certificate file %s: %v", filename, err)
				os.Exit(1)
			}
		}

		// Update config with certificate paths
		cfg.Security.CACert = filepath.Join(certsDir, "ca.crt")
		cfg.Security.ClientCert = filepath.Join(certsDir, "client.crt")
		cfg.Security.ClientKey = filepath.Join(certsDir, "client.key")

		// Save the updated config
		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully logged in to GiraffeCloud")
		logger.Info("API server: %s:%d", cfg.API.Host, cfg.API.Port)
		logger.Info("Tunnel server: %s:%d", cfg.Server.Host, cfg.Server.Port)
		logger.Info("Certificates stored in: %s", certsDir)
		logger.Info("Run 'giraffecloud connect' to establish a tunnel connection")
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tunnel connection status and statistics",
	Long:  `Display the current status of the tunnel connection, including connection state, retry count, and other statistics.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Note: This is a simplified status check
		// In a real implementation, you'd want to connect to a running tunnel process
		// or read status from a shared file/socket

		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		logger.Info("=== GiraffeCloud Tunnel Status ===")
		logger.Info("Configuration:")
		logger.Info("  Domain: %s", cfg.Domain)
		logger.Info("  Local Port: %d", cfg.LocalPort)
		logger.Info("  Server: %s:%d", cfg.Server.Host, cfg.Server.Port)

		if cfg.Security.CACert != "" {
			logger.Info("  CA Certificate: %s", cfg.Security.CACert)
		}
		if cfg.Security.ClientCert != "" {
			logger.Info("  Client Certificate: %s", cfg.Security.ClientCert)
		}

		// Check if certificates exist
		if cfg.Security.CACert != "" {
			if _, err := os.Stat(cfg.Security.CACert); os.IsNotExist(err) {
				logger.Info("  Status: ❌ CA certificate not found - run 'giraffecloud login' first")
				return
			}
		}

		if cfg.Security.ClientCert != "" {
			if _, err := os.Stat(cfg.Security.ClientCert); os.IsNotExist(err) {
				logger.Info("  Status: ❌ Client certificate not found - run 'giraffecloud login' first")
				return
			}
		}

		// Try to connect briefly to check server availability
		logger.Info("Checking server connectivity...")

		// Create TLS config
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.Security.InsecureSkipVerify,
		}

		// Load CA certificate if provided
		if cfg.Security.CACert != "" {
			caCert, err := os.ReadFile(cfg.Security.CACert)
			if err != nil {
				logger.Info("  Status: ❌ Failed to read CA certificate: %v", err)
				return
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				logger.Info("  Status: ❌ Failed to parse CA certificate")
				return
			}

			tlsConfig.RootCAs = caCertPool
		}

		// Load client certificate if provided
		if cfg.Security.ClientCert != "" && cfg.Security.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(cfg.Security.ClientCert, cfg.Security.ClientKey)
			if err != nil {
				logger.Info("  Status: ❌ Failed to load client certificate: %v", err)
				return
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		dialer := &net.Dialer{
			Timeout: 5 * time.Second,
		}

		conn, err := tls.DialWithDialer(dialer, "tcp", serverAddr, tlsConfig)
		if err != nil {
			logger.Info("  Status: ❌ Cannot connect to server: %v", err)
			logger.Info("  Suggestion: Check if server is running or try 'giraffecloud connect'")
			return
		}
		conn.Close()

		logger.Info("  Status: ✅ Server is reachable")
		logger.Info("  Suggestion: Run 'giraffecloud connect' to establish tunnel")
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
	rootCmd.AddCommand(statusCmd)

	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)

	// Add host flags to connect command
	connectCmd.Flags().String("tunnel-host", "", "Tunnel host to connect to (default: tunnel.giraffecloud.xyz)")
	connectCmd.Flags().Int("tunnel-port", 4443, "Tunnel port to connect to (default: 4443)")
	connectCmd.Flags().Int("local-port", 0, "Local port to forward requests to (optional, defaults to port configured on server)")

	// Add host flags to login command
	loginCmd.Flags().String("api-host", "", "API host for login/certificates (default: api.giraffecloud.xyz)")
	loginCmd.Flags().Int("api-port", 443, "API port for login/certificates (default: 443)")
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