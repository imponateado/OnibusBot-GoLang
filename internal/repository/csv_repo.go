package repository

import (
	"os"
	"strings"
	"github.com/leoteodoro/onibus-bot-go/internal/domain"
)

type csvGroupRepository struct {
	path string
}

func NewCSVGroupRepository(path string) domain.GroupRepository {
	return &csvGroupRepository{path: path}
}

func (r *csvGroupRepository) Load() ([]domain.BusGroup, error) {
	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var groups []domain.BusGroup
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ";")
		if len(parts) >= 2 {
			name := strings.ToUpper(strings.TrimSpace(parts[0]))
			busLines := strings.Split(parts[1], ",")
			for i := range busLines {
				busLines[i] = strings.TrimSpace(busLines[i])
			}
			groups = append(groups, domain.BusGroup{
				Name:  name,
				Lines: busLines,
			})
		}
	}
	return groups, nil
}
