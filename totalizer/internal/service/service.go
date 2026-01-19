package service

import (
	"github.com/aelhady03/sumflow/totalizer/internal/storage"
)

type TotalizerService struct {
	storage *storage.PostgresStorage
}

func NewTotalizerService(storage *storage.PostgresStorage) *TotalizerService {
	return &TotalizerService{
		storage: storage,
	}
}

func (t *TotalizerService) Get() (int, error) {
	return t.storage.Load()
}
