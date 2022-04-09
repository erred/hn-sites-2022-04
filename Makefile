.PHONY: stage1
stage1:
	CGO_ENABLED=0 go run -trimpath -ldflags='-s -w' ./cmd/stage1

.PHONY: stage2
stage2:
	CGO_ENABLED=0 go run -trimpath -ldflags='-s -w' ./cmd/stage2

.PHONY: stage3
stage3:
	CGO_ENABLED=0 go run -trimpath -ldflags='-s -w' ./cmd/stage3
