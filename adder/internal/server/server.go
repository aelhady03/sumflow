package server

import (
	"context"

	"github.com/aelhady03/sumflow/adder/internal/service"
	sumpb "github.com/aelhady03/sumflow/adder/proto/sum"
)

type SumNumbersServer struct {
	sumpb.UnimplementedSumNumbersServiceServer
	service *service.AdderService
}

func NewSumNumbersServer(service *service.AdderService) *SumNumbersServer {
	return &SumNumbersServer{service: service}
}

func (s *SumNumbersServer) SumNumbers(ctx context.Context, r *sumpb.SumNumbersRequest) (*sumpb.SumNumbersResponse, error) {
	x, y := r.X, r.Y
	sum, err := s.service.Add(int(x), int(y))
	if err != nil {
		return nil, err
	}
	return &sumpb.SumNumbersResponse{Sum: int32(sum)}, nil
}
