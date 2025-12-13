// cobra への移植版 CLI 実装
package allino

import (
	"fmt"
	"os"

	_ "embed"

	"github.com/spf13/cobra"
)

type CLI struct {
	Command *cobra.Command

	config    *Config
	configDir string
	workDir   string
	bind      string
}

func NewCLI(config *Config) *CLI {
	cli := &CLI{config: config}

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
	cli.Command = rootCmd

	rootCmd.PersistentFlags().StringVarP(&cli.configDir, "config-dir", "c", "", "Set config directory path")
	rootCmd.PersistentFlags().StringVarP(&cli.workDir, "work-dir", "w", "", "Set working directory path")
	rootCmd.PersistentFlags().StringVarP(&cli.bind, "bind", "b", "", "Set HTTP server bind address")

	if !isDisabled("serve") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "serve",
			Short: "Start the web server",
			Run: func(cmd *cobra.Command, args []string) {
				s := cli.InitServer()
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
				s := cli.InitServer()
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
				s := cli.InitServer()
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
				s := cli.InitServer()
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
				s := cli.InitServer()
				fmt.Println("Allino v" + s.Config.Version)
			},
		})
	}

	if !isDisabled("keygen") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "keygen",
			Short: "Generate secrets.config.json file",
			Run: func(cmd *cobra.Command, args []string) {
				s := cli.InitServer()
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
				s := cli.InitServer()
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

	return cli
}

func (cli *CLI) InitServer() *Server {

	if cli.workDir != "" {
		os.Chdir(cli.workDir)
	}

	if cli.config == nil {
		cli.config = &Config{}
	}

	oninit := cli.config.OnInit
	cli.config.OnInit = func(s *Server) error {
		if cli.configDir != "" {
			cli.config.ConfigDir = cli.configDir
		}
		if cli.bind != "" {
			cli.config.Bind = cli.bind
		}
		if oninit != nil {
			return oninit(s)
		}
		return nil
	}

	s, err := NewServer(cli.config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return s
}

func RunCLI(config *Config) {
	cli := NewCLI(config)
	if err := cli.Command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
