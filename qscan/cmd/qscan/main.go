package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/qualys/qscan/internal/config"
	"github.com/qualys/qscan/internal/container"
	"github.com/qualys/qscan/internal/embedded"
	"github.com/qualys/qscan/internal/output"
	"github.com/qualys/qscan/internal/scanner"
)

var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	cfgFile   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "qscan",
		Short: "Qualys QScanner wrapper for Apptainer/Singularity containers",
		Long: `qscan wraps the Qualys qscanner for scanning Apptainer/Singularity containers.

It supports scanning SIF files and running containers, with automatic filesystem
extraction and OS detection.`,
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.config/qscan/config.yaml)")
	rootCmd.PersistentFlags().StringP("token", "t", "", "Qualys access token")
	rootCmd.PersistentFlags().StringP("pod", "p", "", "Qualys POD (e.g., US1, US2, EU1)")
	rootCmd.PersistentFlags().String("scan-types", "", "Scan types: pkg,fileinsight,secret")
	rootCmd.PersistentFlags().String("mode", "", "Mode: get-report, scan-only, inventory-only, evaluate-policy")
	rootCmd.PersistentFlags().String("format", "", "Output formats: json,spdx,cyclonedx,sarif")
	rootCmd.PersistentFlags().StringP("output-dir", "o", "", "Output directory for reports")
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON (for automation)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress progress output")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose logging")

	viper.BindPFlag("qualys.token", rootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("qualys.pod", rootCmd.PersistentFlags().Lookup("pod"))
	viper.BindPFlag("defaults.scan_types", rootCmd.PersistentFlags().Lookup("scan-types"))
	viper.BindPFlag("defaults.mode", rootCmd.PersistentFlags().Lookup("mode"))
	viper.BindPFlag("defaults.format", rootCmd.PersistentFlags().Lookup("format"))
	viper.BindPFlag("defaults.output_dir", rootCmd.PersistentFlags().Lookup("output-dir"))

	cobra.OnInitialize(func() {
		config.InitConfig(cfgFile)
	})

	rootCmd.AddCommand(newSifCmd())
	rootCmd.AddCommand(newRunningCmd())
	rootCmd.AddCommand(newImageCmd())
	rootCmd.AddCommand(newRepoCmd())
	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newSifCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sif <path.sif> [flags]",
		Short: "Scan SIF file(s)",
		Long:  `Scan one or more Apptainer/Singularity SIF files for vulnerabilities.`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  runSifScan,
	}
	return cmd
}

func runSifScan(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	if err := cfg.Validate(); err != nil {
		return err
	}

	qscannerPath, err := embedded.ExtractQScanner()
	if err != nil {
		return fmt.Errorf("failed to extract qscanner: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	opts := scanner.ScanOptions{
		Token:       cfg.Qualys.Token,
		Pod:         cfg.Qualys.Pod,
		ScanTypes:   cfg.GetScanTypes(),
		Mode:        cfg.GetMode(),
		Format:      cfg.GetFormat(),
		OutputDir:   cfg.GetOutputDir(),
		QScannerPath: qscannerPath,
		Quiet:       quiet,
	}

	var results []*scanner.ScanResult
	for _, sifPath := range args {
		if !quiet {
			fmt.Printf("Scanning: %s\n", sifPath)
		}

		result, err := scanner.ScanSIF(cmd.Context(), sifPath, runtime, opts)
		if err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "Error scanning %s: %v\n", sifPath, err)
			}
			result = &scanner.ScanResult{
				Target:   sifPath,
				Type:     "sif",
				ExitCode: 1,
				Error:    err.Error(),
			}
		}
		results = append(results, result)
	}

	if jsonOutput {
		return output.PrintJSON(results)
	}

	for _, result := range results {
		output.PrintTable(result)
	}

	for _, result := range results {
		if result.ExitCode != 0 {
			os.Exit(result.ExitCode)
		}
	}

	return nil
}

func newRunningCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "running <pid|name> [flags]",
		Short: "Scan running container",
		Long:  `Scan a running Apptainer/Singularity container by PID or name.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRunningScan,
	}
	cmd.Flags().Bool("list", false, "List running containers")
	return cmd
}

func runRunningScan(cmd *cobra.Command, args []string) error {
	listFlag, _ := cmd.Flags().GetBool("list")

	if listFlag {
		return container.ListContainers()
	}

	if len(args) == 0 {
		return fmt.Errorf("target (PID or name) required, or use --list")
	}

	cfg := config.Get()
	if err := cfg.Validate(); err != nil {
		return err
	}

	qscannerPath, err := embedded.ExtractQScanner()
	if err != nil {
		return fmt.Errorf("failed to extract qscanner: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	opts := scanner.ScanOptions{
		Token:       cfg.Qualys.Token,
		Pod:         cfg.Qualys.Pod,
		ScanTypes:   cfg.GetScanTypes(),
		Mode:        cfg.GetMode(),
		Format:      cfg.GetFormat(),
		OutputDir:   cfg.GetOutputDir(),
		QScannerPath: qscannerPath,
		Quiet:       quiet,
	}

	result, err := scanner.ScanRunning(cmd.Context(), args[0], opts)
	if err != nil {
		if jsonOutput {
			result = &scanner.ScanResult{
				Target:   args[0],
				Type:     "running",
				ExitCode: 1,
				Error:    err.Error(),
			}
			return output.PrintJSON([]*scanner.ScanResult{result})
		}
		return err
	}

	if jsonOutput {
		return output.PrintJSON([]*scanner.ScanResult{result})
	}

	output.PrintTable(result)

	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}

	return nil
}

func newImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image <ref> [flags]",
		Short: "Scan registry image (passthrough to qscanner)",
		Long:  `Scan a container image from a registry. This passes through to qscanner image command.`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  runImageScan,
	}
	return cmd
}

func runImageScan(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	if err := cfg.Validate(); err != nil {
		return err
	}

	qscannerPath, err := embedded.ExtractQScanner()
	if err != nil {
		return fmt.Errorf("failed to extract qscanner: %w", err)
	}

	opts := scanner.ScanOptions{
		Token:       cfg.Qualys.Token,
		Pod:         cfg.Qualys.Pod,
		ScanTypes:   cfg.GetScanTypes(),
		Mode:        cfg.GetMode(),
		Format:      cfg.GetFormat(),
		OutputDir:   cfg.GetOutputDir(),
		QScannerPath: qscannerPath,
	}

	return scanner.RunDirect(cmd.Context(), "image", args, opts)
}

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <path> [flags]",
		Short: "Scan source code repository (passthrough to qscanner)",
		Long:  `Scan a source code repository. This passes through to qscanner repo command.`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  runRepoScan,
	}
	return cmd
}

func runRepoScan(cmd *cobra.Command, args []string) error {
	cfg := config.Get()
	if err := cfg.Validate(); err != nil {
		return err
	}

	qscannerPath, err := embedded.ExtractQScanner()
	if err != nil {
		return fmt.Errorf("failed to extract qscanner: %w", err)
	}

	opts := scanner.ScanOptions{
		Token:       cfg.Qualys.Token,
		Pod:         cfg.Qualys.Pod,
		ScanTypes:   cfg.GetScanTypes(),
		Mode:        cfg.GetMode(),
		Format:      cfg.GetFormat(),
		OutputDir:   cfg.GetOutputDir(),
		QScannerPath: qscannerPath,
	}

	return scanner.RunDirect(cmd.Context(), "repo", args, opts)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("qscan version %s\n", Version)
			fmt.Printf("Build time: %s\n", BuildTime)

			qscannerPath, err := embedded.ExtractQScanner()
			if err != nil {
				fmt.Printf("Embedded qscanner: extraction error - %v\n", err)
				return
			}
			fmt.Printf("Embedded qscanner: %s\n", qscannerPath)
		},
	}
}
