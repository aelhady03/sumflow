package service

import "github.com/aelhady03/sumflow/adder/internal/kafka"

type AdderService struct {
	producer kafka.Producer
}

func NewAdderService(producer kafka.Producer) *AdderService {
	return &AdderService{
		producer: producer,
	}
}

func (a *AdderService) Add(x, y int) (int, error) {
	sum := x + y
	return sum, a.producer.Publish(sum)
}
