package conf

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff"
)

const name = "wrtag"

var (
	PathFormat    string
	ResearchLinks ResearchLinkConf
)

func Parse() {
	flag.String("config-path", configPath, "path config file")
	flag.StringVar(&PathFormat, "path-format", "", "path format")
	flag.Var(&ResearchLinks, "research-link", "research link")

	if err := ff.Parse(flag.CommandLine, os.Args[1:],
		ff.WithAllowMissingConfigFile(true),
		ff.WithEnvVarPrefix(strings.ToUpper(name)),
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}
}

var (
	userConfig, _ = os.UserConfigDir()
	configPath    = filepath.Join(userConfig, name, "config")
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
