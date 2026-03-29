package repository

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
)

type prefsData struct {
	LowMode         map[string]bool `json:"low_mode"`
	BroadcastOptOut map[string]bool `json:"broadcast_optout"`
}

type jsonUserPrefsRepository struct {
	path string
	mu   sync.Mutex
}

func NewJSONUserPrefsRepository(path string) *jsonUserPrefsRepository {
	return &jsonUserPrefsRepository{path: path}
}

func (r *jsonUserPrefsRepository) load() prefsData {
	data := prefsData{
		LowMode:         make(map[string]bool),
		BroadcastOptOut: make(map[string]bool),
	}

	raw, err := os.ReadFile(r.path)
	if err != nil {
		return data
	}

	// Tenta ler no formato novo (estruturado)
	if err := json.Unmarshal(raw, &data); err != nil {
		// Fallback: tenta ler formato antigo (map[string]bool direto = low mode)
		var oldFormat map[string]bool
		if err := json.Unmarshal(raw, &oldFormat); err == nil {
			data.LowMode = oldFormat
		}
	}

	if data.LowMode == nil {
		data.LowMode = make(map[string]bool)
	}
	if data.BroadcastOptOut == nil {
		data.BroadcastOptOut = make(map[string]bool)
	}

	return data
}

func (r *jsonUserPrefsRepository) save(data prefsData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, raw, 0644)
}

func (r *jsonUserPrefsRepository) SaveLowMode(lowModeUsers map[int64]bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data := r.load()
	data.LowMode = int64MapToString(lowModeUsers)
	return r.save(data)
}

func (r *jsonUserPrefsRepository) LoadLowMode() (map[int64]bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data := r.load()
	return stringMapToInt64(data.LowMode), nil
}

func (r *jsonUserPrefsRepository) SaveBroadcastOptOut(optOutUsers map[int64]bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data := r.load()
	data.BroadcastOptOut = int64MapToString(optOutUsers)
	return r.save(data)
}

func (r *jsonUserPrefsRepository) LoadBroadcastOptOut() (map[int64]bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data := r.load()
	return stringMapToInt64(data.BroadcastOptOut), nil
}

func int64MapToString(m map[int64]bool) map[string]bool {
	result := make(map[string]bool, len(m))
	for k, v := range m {
		result[strconv.FormatInt(k, 10)] = v
	}
	return result
}

func stringMapToInt64(m map[string]bool) map[int64]bool {
	result := make(map[int64]bool, len(m))
	for k, v := range m {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		result[id] = v
	}
	return result
}
