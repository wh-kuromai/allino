// cobra への移植版 CLI 実装
package allino

import (
	"fmt"
	"os"

	_ "embed"

	"github.com/spf13/cobra"
)

func NewCLI(config *Config) *cobra.Command {
	var configDir, workDir, bind string

	getServer := func() *Server {
		if workDir != "" {
			os.Chdir(workDir)
		}

		if config == nil {
			config = &Config{}
		}

		oninit := config.OnInit
		config.OnInit = func(s *Server) error {
			if configDir != "" {
				config.ConfigDir = configDir
			}
			if bind != "" {
				config.Bind = bind
			}
			if oninit != nil {
				return oninit(s)
			}
			return nil
		}

		s, err := NewServer(config)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return s
	}

	isDisabled := func(cmd string) bool {
		if config != nil {
			for _, name := range config.DisabledCommands {
				if name == cmd {
					return true
				}
			}
		}
		return false
	}

	rootCmd := &cobra.Command{
		Use:   "allino",
		Short: "allino - AI-first web framework server",
		//Long:  helptemplate,
	}

	rootCmd.PersistentFlags().StringVarP(&configDir, "config-dir", "c", "", "Set config directory path")
	rootCmd.PersistentFlags().StringVarP(&workDir, "work-dir", "w", "", "Set working directory path")
	rootCmd.PersistentFlags().StringVarP(&bind, "bind", "b", "", "Set HTTP server bind address")

	if !isDisabled("serve") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "serve",
			Short: "Start the web server",
			Run: func(cmd *cobra.Command, args []string) {
				s := getServer()
				s.RegisterAllTypedHandler()
				s.Serve()
			},
		})
	}

	if !isDisabled("proxyvisor-plugin") {
		rootCmd.AddCommand(&cobra.Command{
			Use:    "plugin-start",
			Short:  "Start the server in plugin mode",
			Hidden: true,
			Run: func(cmd *cobra.Command, args []string) {
				config.ConfigDir = os.Getenv("PROXYVISOR_PLUGIN_CONFIG_DIR")
				config.Bind = os.Getenv("PROXYVISOR_PLUGIN_ADDRESS")
				s := getServer()
				s.RegisterAllTypedHandler()
				s.Serve()
			},
		})
	}

	if !isDisabled("openapi") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "openapi",
			Short: "Generate OpenAPI YAML",
			Run: func(cmd *cobra.Command, args []string) {
				s := getServer()
				s.RegisterAllTypedHandler()
				printOpenAPI(s)
			},
		})
	}

	if !isDisabled("route") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "route",
			Short: "Print registered routes",
			Run: func(cmd *cobra.Command, args []string) {
				s := getServer()
				s.RegisterAllTypedHandler()
				printRoute(s)
			},
		})
	}

	if !isDisabled("version") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "version",
			Short: "Print version info",
			Run: func(cmd *cobra.Command, args []string) {
				s := getServer()
				fmt.Println("Allino v" + s.Config.Version)
			},
		})
	}

	if !isDisabled("keygen") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "keygen",
			Short: "Generate secrets.config.json file",
			Run: func(cmd *cobra.Command, args []string) {
				s := getServer()
				cliKeygen(s)
			},
		})
	}

	if !isDisabled("encrypt") {
		encryptFile := ""
		encryptCmd := &cobra.Command{
			Use:   "encrypt",
			Short: "Encrypt config file",
			RunE: func(cmd *cobra.Command, args []string) error {
				s := getServer()
				return cliEncrypt(s.envPrefix(), encryptFile)
			},
		}
		encryptCmd.Flags().StringVarP(&encryptFile, "file", "f", "", "Set YAML config file path")
		encryptCmd.MarkFlagRequired("file")
		rootCmd.AddCommand(encryptCmd)
	}

	for _, ext := range extensionList {
		opt := ext.ExtOption()
		for _, cmd := range opt.CLICommands {
			rootCmd.AddCommand(cmd)
		}
	}

	if config != nil {
		if config.AppName != "" {
			rootCmd.Use = config.AppName
		}
		if config.Description != "" {
			rootCmd.Short = config.Description
		}
	}

	return rootCmd
}

func RunCLI(config *Config) {
	rootCmd := NewCLI(config)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
