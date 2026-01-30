package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Qualys   QualysConfig   `mapstructure:"qualys"`
	Defaults DefaultsConfig `mapstructure:"defaults"`
}

type QualysConfig struct {
	Token string `mapstructure:"token"`
	Pod   string `mapstructure:"pod"`
}

type DefaultsConfig struct {
	ScanTypes string `mapstructure:"scan_types"`
	Mode      string `mapstructure:"mode"`
	Format    string `mapstructure:"format"`
	OutputDir string `mapstructure:"output_dir"`
}

var cfg *Config

func InitConfig(cfgFile string) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".config", "qscan"))
		}
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("")
	viper.BindEnv("qualys.token", "QUALYS_ACCESS_TOKEN")
	viper.BindEnv("qualys.pod", "QUALYS_POD")
	viper.BindEnv("defaults.scan_types", "SCAN_TYPES")
	viper.BindEnv("defaults.output_dir", "OUTPUT_DIR")

	viper.SetDefault("defaults.scan_types", "pkg,fileinsight")
	viper.SetDefault("defaults.mode", "get-report")
	viper.SetDefault("defaults.output_dir", "./reports")

	viper.ReadInConfig()

	cfg = &Config{}
	viper.Unmarshal(cfg)
}

func Get() *Config {
	if cfg == nil {
		InitConfig("")
	}
	return cfg
}

func (c *Config) Validate() error {
	mode := c.GetMode()
	if mode == "inventory-only" {
		return nil
	}

	if c.Qualys.Token == "" {
		return fmt.Errorf("Qualys access token required. Set via --token, QUALYS_ACCESS_TOKEN, or config file")
	}
	if c.Qualys.Pod == "" {
		return fmt.Errorf("Qualys POD required. Set via --pod, QUALYS_POD, or config file")
	}
	return nil
}

func (c *Config) GetScanTypes() string {
	if c.Defaults.ScanTypes != "" {
		return c.Defaults.ScanTypes
	}
	return "pkg,fileinsight"
}

func (c *Config) GetMode() string {
	if c.Defaults.Mode != "" {
		return c.Defaults.Mode
	}
	return "get-report"
}

func (c *Config) GetFormat() string {
	return c.Defaults.Format
}

func (c *Config) GetOutputDir() string {
	if c.Defaults.OutputDir != "" {
		return c.Defaults.OutputDir
	}
	return "./reports"
}
