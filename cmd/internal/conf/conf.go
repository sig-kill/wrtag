package conf

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"go.senan.xyz/flagconf"
)

const name = "wrtag"

var (
	PathFormat    string
	ResearchLinks ResearchLinkConf
)

func Parse() {
	flag.CommandLine.Init("wrtag", flag.ExitOnError)
	flag.StringVar(&PathFormat, "path-format", "", "path format")
	flag.Var(&ResearchLinks, "research-link", "research link")

	configPath := flag.String("config-path", defaultConfigPath, "path config file")

	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)
}

var (
	userConfig, _     = os.UserConfigDir()
	defaultConfigPath = filepath.Join(userConfig, name, "config")
)

type ResearchLinkConf []ResearchLinkTemplate
type ResearchLinkTemplate struct {
	Name     string
	Template string
}

func (sls ResearchLinkConf) String() string {
	var names []string
	for _, sl := range sls {
		names = append(names, sl.Name)
	}
	return strings.Join(names, ", ")
}

func (sls *ResearchLinkConf) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	*sls = append(*sls, ResearchLinkTemplate{Name: name, Template: value})
	return nil
}
