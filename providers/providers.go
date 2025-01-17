package providers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"go.mondoo.com/cnquery/cli/config"
	"go.mondoo.com/cnquery/providers/plugin"
	"go.mondoo.com/cnquery/resources"
)

var (
	SystemPath string
	HomePath   string
)

func init() {
	SystemPath = config.SystemDataPath("providers")
	if os.Geteuid() != 0 {
		HomePath, _ = config.HomePath("providers")
	}
}

type Providers map[string]*Provider

type Provider struct {
	*plugin.Provider
	Schema *resources.Schema
	Path   string
}

func List() (Providers, error) {
	local := listPaths()
	var res Providers = make(map[string]*Provider, len(local))
	for _, v := range local {
		if err := v.LoadJson(); err != nil {
			return nil, err
		}
		res[v.ID] = v
	}

	// we add builtin ones here, possibly overriding providers in paths
	for name, x := range builtinProviders {
		res[name] = &Provider{
			Provider: x.Config,
		}
	}

	// useful for caching; even if the structure gets updated with new providers
	Coordinator.Providers = res
	return res, nil
}

func listPaths() Providers {
	// This really shouldn't happen, but just in case it does...
	if SystemPath == "" && HomePath == "" {
		log.Error().Msg("can't find any paths for providers, none are configured")
		return nil
	}

	sysOk := config.ProbeDir(SystemPath)
	homeOk := config.ProbeDir(HomePath)
	if !sysOk && !homeOk {
		msg := log.Warn()
		if SystemPath != "" {
			msg = msg.Str("system-path", SystemPath)
		}
		if HomePath != "" {
			msg = msg.Str("home-path", HomePath)
		}
		msg.Msg("no provider paths exist")
		return nil
	}

	providers := map[string]*Provider{}

	if sysOk {
		err := findProviders(SystemPath, providers)
		if err != nil {
			log.Warn().Str("path", SystemPath).Msg("failed to get providers from system path")
		}
	}

	if homeOk {
		err := findProviders(HomePath, providers)
		if err != nil {
			log.Warn().Str("path", HomePath).Msg("failed to get providers from home path")
		}
	}

	return providers
}

func isOverlyPermissive(path string) (bool, error) {
	stat, err := config.AppFs.Stat(path)
	if err != nil {
		return true, errors.New("failed to analyze " + path)
	}

	mode := stat.Mode()
	if mode&0o022 != 0 {
		return true, nil
	}

	return false, nil
}

func findProviders(path string, res map[string]*Provider) error {
	overlyPermissive, err := isOverlyPermissive(path)
	if err != nil {
		return err
	}
	if overlyPermissive {
		return errors.New("path is overly permissive, make sure it is not writable to others or the group: " + path)
	}

	log.Debug().Str("path", path).Msg("searching providers in path")
	files, err := afero.ReadDir(config.AppFs, path)
	if err != nil {
		return err
	}

	candidates := map[string]struct{}{}
	otherFiles := map[string]struct{}{}
	for i := range files {
		file := files[i]
		if !file.Mode().IsRegular() {
			continue
		}

		name := file.Name()
		if strings.IndexByte(name, '.') == -1 {
			candidates[name] = struct{}{}
			continue
		}
		if strings.HasSuffix(name, ".json") {
			otherFiles[name] = struct{}{}
		}
	}

	for name := range candidates {
		if _, ok := otherFiles[name+".json"]; !ok {
			continue
		}

		res[name] = &Provider{
			Path: filepath.Join(path, name),
		}
	}

	return nil
}

// This is the default installation source for core providers.
const upstreamURL = "https://releases.mondoo.com/providers/{NAME}/{VERSION}/{NAME}_{VERSION}_{BUILD}.tar.xz"

func Install(name string) (*Provider, error) {
	panic("INSTALL")
}

func (p *Provider) LoadJson() error {
	path := p.Path + ".json"
	res, err := afero.ReadFile(config.AppFs, path)
	if err != nil {
		return errors.New("failed to read provider json from " + path + ": " + err.Error())
	}

	if err := json.Unmarshal(res, &p.Provider); err != nil {
		return errors.New("failed to parse provider json from " + path + ": " + err.Error())
	}
	return nil
}

func (p *Provider) LoadResources() error {
	path := p.Path + ".resources.json"
	res, err := afero.ReadFile(config.AppFs, path)
	if err != nil {
		return errors.New("failed to read provider resources json from " + path + ": " + err.Error())
	}

	if err := json.Unmarshal(res, &p.Schema); err != nil {
		return errors.New("failed to parse provider resources json from " + path + ": " + err.Error())
	}
	return nil
}

func (p Providers) ForConnection(name string) *Provider {
	for _, provider := range p {
		for i := range provider.Connectors {
			connector := provider.Connectors[i]
			if connector.Name == name {
				return provider
			}
		}
	}

	return nil
}

func (p Providers) Add(nu *Provider) {
	if nu != nil {
		p[nu.Name] = nu
	}
}
