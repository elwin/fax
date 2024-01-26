push:
	docker buildx build . --tag ghcr.io/elwin/fax:latest --platform linux/amd64,linux/arm64 --push