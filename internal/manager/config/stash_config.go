package config

import (
	"path/filepath"

	"github.com/stashapp/stash/pkg/fsutil"
)

// Stash configuration details
type StashConfigInput struct {
	Path         string `json:"path"`
	ExcludeVideo bool   `json:"excludeVideo"`
	ExcludeImage bool   `json:"excludeImage"`
	Restricted   bool   `json:"restricted"`
}

type StashConfig struct {
	Path         string `json:"path"`
	ExcludeVideo bool   `json:"excludeVideo"`
	ExcludeImage bool   `json:"excludeImage"`
	Restricted   bool   `json:"restricted"`
}

type StashConfigs []*StashConfig

func (s StashConfigs) GetStashFromPath(path string) *StashConfig {
	for _, f := range s {
		if fsutil.IsPathInDir(f.Path, filepath.Dir(path)) {
			return f
		}
	}
	return nil
}

func (s StashConfigs) GetStashFromDirPath(dirPath string) *StashConfig {
	for _, f := range s {
		if fsutil.IsPathInDir(f.Path, dirPath) {
			return f
		}
	}
	return nil
}

func (s StashConfigs) Paths() []string {
	paths := make([]string, len(s))
	for i, c := range s {
		// #6618 - clean the path to ensure comparison works correctly
		paths[i] = filepath.Clean(c.Path)
	}
	return paths
}

// IsPathRestricted returns true if the provided path is inside any restricted
// stash path.
//
// Restricted paths always take precedence over non-restricted paths, regardless
// of parent/child path relationships.
func (s StashConfigs) IsPathRestricted(path string) bool {
	for _, c := range s {
		if c.Restricted && fsutil.IsPathInDir(c.Path, path) {
			return true
		}
	}

	return false
}
