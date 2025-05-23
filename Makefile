sirup: $(wildcard **/*.go go.*)
	go build .

.PHONY: release
release:
	CGO_ENABLED=0 go build -ldflags '-s -w'

.PHONY: image
image: release
	docker image build --pull -t rokeller/sirup .
