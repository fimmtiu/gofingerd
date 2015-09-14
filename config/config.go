package config

import "flag"
import "fmt"
import "os"
import "time"
import "github.com/BurntSushi/toml"

type Config struct {
	AllowQueryForwarding bool      // FIXME: Not implemented yet
	AllowUserListing bool
	AllowApproximateSearch bool
	Port int
	NetworkTimeout time.Duration   // Specified in seconds
	ConfigFile string
}

func ReadConfig() Config {
	conf := defaultConfig()
	parseOptions(&conf)
	readConfigFile(&conf)

	// Sadly, Port has to be an int (instead of a uint16) for flag.IntVar to
	// work, so we need to manually validate the user-submitted value here.
	if conf.Port < 0 || conf.Port > 65535 {
		panic(fmt.Sprintf("\"%d\" is an invalid port number!", conf.Port))
	}
	conf.NetworkTimeout *= time.Second
	return conf
}

// The default setup is very restrictive, security-wise.
func defaultConfig() (Config) {
	return Config{
		false,
		false,
		false,
		79,
		30,
		"",
	}
}

// Modify an existing Config object with options from the command line.
func parseOptions(conf *Config) {
	// Parse the command-line arguments.
	flag.IntVar(&conf.Port, "p", conf.Port, "Port to listen on")
	flag.StringVar(&conf.ConfigFile, "c", conf.ConfigFile, "Path to the config file")
	// FIXME: Add more command-line config options.
	flag.Parse()
}

// Modify an existing Config object with the settings in a config file. If
// the config file wasn't supplied; try "gofingerd.conf"; if that doesn't
// exist, ignore the whole matter.
func readConfigFile(conf *Config) {
	filename := conf.ConfigFile
	if len(filename) == 0 {
		filename = "gofingerd.conf"
	}
	_, err := toml.DecodeFile(filename, &conf)
	if err != nil && !os.IsNotExist(err) && len(conf.ConfigFile) > 0 {
		panic(fmt.Sprintf("Can't read config file \"%s\": %s", conf.ConfigFile, err.Error()))
	}
}
