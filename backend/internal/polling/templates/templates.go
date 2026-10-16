package templates

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed polling_*.json
var templateFS embed.FS

// PollingTemplateEnt defines a reusable monitoring template.
type PollingTemplateEnt struct {
	ID        int    `json:"ID"`
	Name      string `json:"Name"`
	Level     string `json:"Level"`
	Type      string `json:"Type"`
	Mode      string `json:"Mode"`
	Params    string `json:"Params"`
	Filter    string `json:"Filter"`
	Extractor string `json:"Extractor"`
	Script    string `json:"Script"`
	Descr     string `json:"Descr"`
	AutoParam string `json:"AutoParam"`
}

var (
	tmplMu      sync.RWMutex
	tmplCacheJa []*PollingTemplateEnt
	tmplCacheEn []*PollingTemplateEnt
)

// LoadTemplates loads embedded polling templates for the specified language.
func LoadTemplates(lang string) ([]*PollingTemplateEnt, error) {
	tmplMu.Lock()
	defer tmplMu.Unlock()

	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")
	if isJa && len(tmplCacheJa) > 0 {
		return tmplCacheJa, nil
	} else if !isJa && len(tmplCacheEn) > 0 {
		return tmplCacheEn, nil
	}

	fileName := "polling_ja.json"
	if !isJa {
		fileName = "polling_en.json"
	}

	data, err := templateFS.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read template file %s: %w", fileName, err)
	}

	var list []PollingTemplateEnt
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("unmarshal template file %s: %w", fileName, err)
	}

	result := make([]*PollingTemplateEnt, len(list))
	for i := range list {
		list[i].ID = i + 1
		result[i] = &list[i]
	}

	if isJa {
		tmplCacheJa = result
	} else {
		tmplCacheEn = result
	}

	return result, nil
}
