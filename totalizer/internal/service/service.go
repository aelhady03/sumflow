package service

import (
	"fmt"

	"github.com/aelhady03/sumflow/totalizer/internal/storage"
)

type TotalizerService struct {
	storage storage.Storage
}

func NewTotalizerService(storage storage.Storage) *TotalizerService {
	return &TotalizerService{
		storage: storage,
	}
}

func (t *TotalizerService) Add(total int) error {
	current, err := t.storage.Load()
	if err != nil {
		return fmt.Errorf("cannot load file storage")
	}

	newTotal := current + total
	return t.storage.Save(newTotal)
}

func (t *TotalizerService) Get() (int, error) {
	return t.storage.Load()
}
