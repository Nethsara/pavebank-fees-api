.PHONY: tidy test-money test-billworkflow test-all

tidy:
	go mod tidy

test-money: tidy
	go test ./bill/money/...

test-billworkflow: tidy
	go test ./bill/billworkflow/...

test-all: tidy 
	make test-money && make test-billworkflow