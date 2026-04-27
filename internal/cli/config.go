package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type config struct {
	Server        string  `json:"server"`
	Version       Version `json:"version"`
	Token         string  `json:"token"`
	Tenant        string  `json:"tenant,omitempty"`
	ClientID      string  `json:"client_id,omitempty"`
	ClientSecret  string  `json:"client_secret,omitempty"`
	TokenEndpoint string  `json:"token_endpoint,omitempty"`
	ExpiresAt     int64   `json:"expires_at,omitempty"` // unix timestamp
}

type Version int

const (
	V_2302 Version = iota + 1
	V_2306
	V_2310
	V_2402
	V_2406
	V_2410
	V_2506
)

var allVersions = []Version{V_2302, V_2306, V_2310, V_2402, V_2406, V_2410, V_2506}
var supportedVersions = []Version{V_2506}

var versionNames = map[Version]string{
	V_2302: "2302",
	V_2306: "2306",
	V_2310: "2310",
	V_2402: "2402",
	V_2406: "2406",
	V_2410: "2410",
	V_2506: "2506",
}

func (v Version) String() string {
	if s, ok := versionNames[v]; ok {
		return s
	}
	return "unknown"
}

func (v Version) IsSupported() bool {
	return slices.Contains(supportedVersions, v)
}

func parseVersion(s string) (Version, error) {
	for v, name := range versionNames {
		if name == s {
			return v, nil
		}
	}
	errStr := &strings.Builder{}
	for _, v := range supportedVersions {
		errStr.WriteString(versionNames[v] + ", ")
	}
	errStrFinal := errStr.String()

	return 0, fmt.Errorf("must be one of: %s", errStrFinal[:len(errStrFinal)-2])
}

// VersionFlag implements pflag.Value interface
type VersionFlag struct {
	Value Version
}

func (f *VersionFlag) String() string {
	if f.Value == 0 {
		return ""
	}
	return versionNames[f.Value]
}

func (f *VersionFlag) Type() string {
	s := &strings.Builder{}
	for i, v := range supportedVersions {
		if i > 0 {
			s.WriteString("|")
		}
		s.WriteString(versionNames[v])
	}
	return s.String()
}

func (f *VersionFlag) Set(s string) error {
	v, err := parseVersion(s)
	if err != nil {
		return err
	}
	f.Value = v
	return nil
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ws1", "config.json"), nil
}

func saveConfig(cfg config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func loadConfig() (config, error) {
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return config{}, err
	}
	defer f.Close()
	var cfg config
	return cfg, json.NewDecoder(f).Decode(&cfg)
}

func deleteConfig() (bool, error) {
	p, err := configPath()
	if err != nil {
		return false, err
	}
	err = os.Remove(p)
	if err != nil {
		return false, err
	}
	return true, nil
}
