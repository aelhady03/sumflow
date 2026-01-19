.PHONY: grpcurl
grpcurl:
	grpcurl -plaintext -d '{"x": 5, "y": 3}' localhost:50051 sum.SumNumbersService/SumNumbers