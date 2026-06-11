// cobra への移植版 CLI 実装
package allino

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/spf13/cobra"
)

type CLI struct {
	Command *cobra.Command

	config        *Config
	configDir     string
	extconfig     []map[string]any
	workDir       string
	bind          string
	set           []string
	debug         bool
	allow_migrate bool
}

var cliServer *Server

func NewCLI(config *Config, extconfig ...map[string]any) *CLI {
	cli := &CLI{config: config, extconfig: extconfig}

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
	}
	cli.Command = rootCmd

	rootCmd.PersistentFlags().StringVarP(&cli.configDir, "config-dir", "c", "", "Set config directory path")
	rootCmd.PersistentFlags().StringVarP(&cli.workDir, "work-dir", "w", "", "Set working directory path")
	rootCmd.PersistentFlags().StringVarP(&cli.bind, "bind", "b", "", "Set HTTP server bind address")
	rootCmd.PersistentFlags().StringArrayVarP(&cli.set, "set", "", nil, "Set cli variable (ex. --set key=value)")
	rootCmd.PersistentFlags().BoolVarP(&cli.debug, "debug", "", false, "Set debug=true")
	rootCmd.PersistentFlags().BoolVarP(&cli.allow_migrate, "allow-migrate", "", false, "Allow SQL auto migration")

	if !isDisabled("serve") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "serve",
			Short: "Start the web server",
			Run: func(cmd *cobra.Command, args []string) {
				s := CLIServer(cmd, args)
				s.RegisterAllTypedHandler()
				s.Serve()
			},
		})
	}

	if !isDisabled("run") {
		injson := ""
		runCmd := &cobra.Command{
			Use:   "run",
			Short: "Run handler",
			Run: func(cmd *cobra.Command, args []string) {
				//pb := NewCLIApp()

				s := CLIServer(cmd, args)
				s.RegisterAllTypedHandler()
				s.serveInitOnly()

				handler, err := find_handler(s, args[0])
				if err != nil {
					fmt.Printf("Error: handler '%s' not found.\n", args[0])
					return
				}

				r := NewRequest(s, nil)
				defer r.do_defer()
				if injson == "" {
					injson = "{}"
				}

				fmt.Printf("Running handler '%s'...\n", handler)
				key, outjson, errjson, syserr := call_direct(s, r, handler, []byte(injson), func(input any) error {

					if input != nil {
						buf, err := json.MarshalIndent(input, "", "  ")
						if err != nil {
							fmt.Printf("Error: %x\n", err)
						}

						fmt.Print("Input:\n")
						fmt.Print(string(buf))
						fmt.Print("\n")
					}
					return nil
				})

				if syserr != nil {
					fmt.Printf("Error: %x\n", syserr)
					return
				}
				fmt.Printf("JobID: %s\n", key)
				if outjson != nil {
					fmt.Print("Output:\n")
					printJSON(outjson)
					fmt.Print("\n")
				}

				if errjson != nil {
					fmt.Print("Error:\n")
					printJSON(errjson)
					fmt.Print("\n")
				}

				if s.jobManager != nil {
					s.jobManager.WaitForJob(s.appctx, s.callSQLStrategy, key, func(doneCount, errCount, total int) {
						fmt.Printf("----> Progress %.0f%% (%d complete, %d error, %d total)\n", 100*float64(doneCount+errCount)/float64(total), doneCount, errCount, total)
						//pb.Progress(float64(doneCount+errCount)/float64(total), doneCount+errCount, total)
					})

				}

				time.Sleep(300 * time.Millisecond)
				//pb.Close()

			},
		}

		runCmd.Flags().StringVarP(&injson, "input", "f", "", "Input JSON (optional)")
		rootCmd.AddCommand(runCmd)
	}

	if !isDisabled("proxyvisor-plugin") {
		rootCmd.AddCommand(&cobra.Command{
			Use:    "plugin-start",
			Short:  "Start the server in plugin mode",
			Hidden: true,
			Run: func(cmd *cobra.Command, args []string) {
				config.ConfigDir = os.Getenv("PROXYVISOR_PLUGIN_CONFIG_DIR")
				config.Bind = os.Getenv("PROXYVISOR_PLUGIN_ADDRESS")
				s := CLIServer(cmd, args)
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
				s := CLIServer(cmd, args)
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
				s := CLIServer(cmd, args)
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
				s := CLIServer(cmd, args)
				fmt.Println("Allino v" + s.Config.Version)
			},
		})
	}

	if !isDisabled("sqlschema") {
		var driver string
		sqlschemaCmd := &cobra.Command{
			Use:   "sqlschema",
			Short: "Print SQL schema for this server",
			Run: func(cmd *cobra.Command, args []string) {
				falseFlag := false
				config.SQL.AllowMigrate = &falseFlag
				s := CLIServer(cmd, args)
				if driver == "" {
					driver = s.Config.SQL.Driver
				}

				for _, ext := range s.extopts {
					if ext.SQLSchema != nil {
						schema := ext.SQLSchema(driver)
						if schema != "" {
							fmt.Print("--------------------------------\n")
							fmt.Print("-- SQL schema: " + ext.Name + " extension\n")
							fmt.Print("--------------------------------\n\n")
							fmt.Print(strings.TrimSpace(schema) + "\n")
						}
					}
				}
			},
		}
		sqlschemaCmd.Flags().StringVarP(&driver, "driver", "", "", "Set SQL Driver name (schema may change depend on schema)")
		rootCmd.AddCommand(sqlschemaCmd)
	}

	if !isDisabled("keygen") {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "keygen",
			Short: "Generate secrets.config.json file",
			Run: func(cmd *cobra.Command, args []string) {
				s := CLIServer(cmd, args)
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
				s := CLIServer(cmd, args)
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

var clionce sync.Once

func CLIServer(cmd *cobra.Command, args []string) *Server {
	clia := cmd.Context().Value("cli")
	cli, ok := clia.(*CLI)
	if !ok {
		panic("invalid context: please use cli.Run()")
		//return nil
	}

	clionce.Do(func() {
		cliServer = cli.initServer()
		if cli.debug {
			cliServer.Config.Debug = true
		}

		if cli.allow_migrate && cliServer.Config.SQL.AllowMigrate == nil {
			trueFlag := true
			cliServer.Config.SQL.AllowMigrate = &trueFlag
		}

		clivar := make(map[string]string)
		cliServer.Config.CliVar = clivar
		for _, v := range cli.set {
			idx := strings.Index(v, "=")
			if idx > 0 {
				clivar[v[:idx]] = v[idx+1:]
			}
		}
		for i, v := range args {
			clivar[strconv.Itoa(i)] = v
		}

	})
	return cliServer
}

func (cli *CLI) initServer() *Server {

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

	s, err := NewServer(cli.config, cli.extconfig...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return s
}

func (cli *CLI) Run() {
	ctx := context.WithValue(context.Background(), "cli", cli)
	if err := cli.Command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func RunCLI(config *Config, extconfig ...map[string]any) {
	cli := NewCLI(config, extconfig...)
	cli.Run()
}
