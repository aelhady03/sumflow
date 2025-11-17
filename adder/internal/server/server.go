package server

import (
	"context"

	sumpb "github.com/aelhady03/sumflow/adder/proto/sum"
)

type SumNumbersServer struct {
	sumpb.UnimplementedSumNumbersServiceServer
}

func NewSumNumbersServer() *SumNumbersServer {
	return &SumNumbersServer{}
}

func (s *SumNumbersServer) SumNumbers(ctx context.Context, r *sumpb.SumNumbersRequest) (*sumpb.SumNumbersResponse, error) {
	x, y := r.X, r.Y
	// TODO: Add the sum to Kafka Log later...
	return &sumpb.SumNumbersResponse{Z: int32(x + y)}, nil
}
