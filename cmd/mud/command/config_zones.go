package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixil98/go-mud/internal/zones"
)

type ZoneConfig struct {
	AssetsPath string `json:"asset_path"`
}

func (zc *ZoneConfig) Validate() error {
	return nil
}

func (zc *ZoneConfig) NewZoneManager() (*zones.ZoneManager, error) {
	zoneSet, err := zc.loadZones()
	if err != nil {
		return nil, fmt.Errorf("loading zones: %w", err)
	}

	return zones.NewZoneManager(zoneSet), nil
}

func (zc *ZoneConfig) loadZones() ([]*zones.Zone, error) {
	var zoneSet []*zones.Zone
	err := filepath.Walk(zc.AssetsPath, func(path string, info os.FileInfo, err error) error {
		// Load all json files in the assets path
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", path, err)
			}

			// Unmarshal the json data into a zone
			var zone zones.Zone
			err = json.Unmarshal(data, &zone)
			if err != nil {
				return fmt.Errorf("unmarshaling zone %s: %w", path, err)
			}
			zoneSet = append(zoneSet, &zone)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking assets path %s: %w", zc.AssetsPath, err)
	}

	return zoneSet, nil
}
