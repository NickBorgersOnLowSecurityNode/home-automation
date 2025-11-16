.DEFAULT_GOAL := help

#help: @ List available tasks on this project
help: 
	@grep -E '[a-zA-Z\.\-]+:.*?@ .*$$' $(MAKEFILE_LIST)| tr -d '#' | sed -E 's/Makefile.//' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

#run-locally: @ Run this project with a local Node Red instance you can interact with and explore
run-locally: run watch-logs

build:
	# Build all three custom images
	cp package.json .automated-rendering/node-red/package.json
	docker build -t node-red-local -f .automated-rendering/node-red/Dockerfile .
	docker build -t node-red-haproxy .automated-rendering/haproxy/
	docker build -t screenshot-capture .automated-rendering/screenshot-capture/

run: build cleanup
	# Create an internal backend network with no gateway
	docker network create node-red-backend --internal
	# Create a network with a gateway
	docker network create node-red-frontend
	# Create the proxy on the frontend network
	docker run --rm -d --network node-red-frontend -p 8080:80 --name node-red-proxy node-red-haproxy
	# Add the backend to the proxy so it can reach the node-red container
	docker network connect node-red-backend node-red-proxy
	# Create the node-red container on the backend network
	docker run -d --user 0:0 -e PORT=80 --network=node-red-backend --name node-red node-red-local

#generate-screenshots: @ Generate screenshots of each tab in the Node Red project
generate-screenshots: build run
	# Hacky sleep to avoid hitting TCP connection refused against node-red container
	sleep 3
	# Start our "test" which pulls the screenshots out of the node-red container
	docker run --rm --network=node-red-backend \
	  --mount type=bind,source=${CURDIR}/.automated-rendering/screenshot-capture/screenshots/,destination=/app/screenshots/ \
	  --name screenshot-capture screenshot-capture npm test
	${MAKE} trim-screenshots
	${MAKE} cleanup

trim-screenshots:
	# Trim our captured screenshots with ImageMagick
	docker run --rm --network=none \
	  --mount type=bind,source=${CURDIR}/.automated-rendering/screenshot-capture/screenshots/,destination=/screenshots/ \
	  --name image-magick-auto-crop --entrypoint=mogrify dpokidov/imagemagick -fuzz 27% -trim +repage /screenshots/*.png

#watch-logs: @ Watch the logs of a running node-red instance
watch-logs:
	docker logs -f node-red

#interactive-node-red: @ Get a shell in a running node-red instance
interactive-node-red:
	docker exec -it node-red bash

#interactive-screenshot-capture: @ Interactive run the screenshot capture script
interactive-screenshot-capture:
	docker run -it --rm --network=node-red-backend \
	  --mount type=bind,source=${CURDIR}/.automated-rendering/screenshot-capture/,destination=/app/ \
	  --name screenshot-capture screenshot-capture

restart:
	docker stop node-red
	docker start node-red

#cleanup: @ Cleanup any remaining containers
cleanup:
	docker stop node-red-proxy || true
	docker stop node-red || true
	docker rm node-red || true
	docker network rm node-red-backend || true
	docker network rm node-red-frontend || true

#run-config-tests: @ Run all available tests of the configuration files
run-config-tests: run-yamllint-hue run-yamllint-music run-spotify-validation-music

run-yamllint-hue: build-config-tester
	docker run --rm --mount type=bind,source=${CURDIR}/configs/hue_config.yaml,destination=/app/hue_config.yaml node-red-config-tester yamllint hue_config.yaml

run-yamllint-music: build-config-tester
	docker run --rm --mount type=bind,source=${CURDIR}/configs/music_config.yaml,destination=/app/music_config.yaml node-red-config-tester yamllint music_config.yaml

run-spotify-validation-music: build-config-tester
	docker run --rm --mount type=bind,source=${CURDIR}/configs/music_config.yaml,destination=/app/music_config.yaml node-red-config-tester python3 -u validate_spotify_uris.py

build-config-tester:
	docker build -t node-red-config-tester ./config-test/

# ============================================================================
# Go Application (homeautomation-go) Targets
# ============================================================================

#build-go: @ Build the Go application binary
build-go:
	cd homeautomation-go && go build -o homeautomation ./cmd/main.go

#test-go: @ Run Go tests with race detection and coverage
test-go:
	cd homeautomation-go && go test ./... -race -v -coverprofile=coverage.out
	cd homeautomation-go && go tool cover -func=coverage.out | grep total

#docker-build-go: @ Build Docker image for the Go application
docker-build-go:
	docker build -t homeautomation:latest ./homeautomation-go/

#docker-run-go: @ Run the Go application in Docker (requires .env file)
docker-run-go: docker-build-go
	@if [ ! -f homeautomation-go/.env ]; then \
		echo "ERROR: homeautomation-go/.env file not found. Copy .env.example and configure it."; \
		exit 1; \
	fi
	docker run --rm -it \
		--name homeautomation \
		--env-file homeautomation-go/.env \
		homeautomation:latest

#docker-push-go: @ Push Go application image to GitHub Container Registry
docker-push-go: docker-build-go
	@echo "Tagging image for GHCR..."
	docker tag homeautomation:latest ghcr.io/nickborgersonlowsecuritynode/home-automation:latest
	@echo "Pushing to ghcr.io/nickborgersonlowsecuritynode/home-automation:latest"
	@echo "Note: You may need to authenticate first with: echo \$$GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin"
	docker push ghcr.io/nickborgersonlowsecuritynode/home-automation:latest

#clean-go: @ Clean Go build artifacts
clean-go:
	rm -f homeautomation-go/homeautomation
	rm -f homeautomation-go/coverage.out

#pre-commit: @ Run all pre-commit checks (style, format, lint, build, tests)
pre-commit:
	@echo "🔍 Running pre-commit checks..."
	@echo ""
	@echo "📝 Step 1/7: Checking gofmt formatting..."
	@cd homeautomation-go && \
	  unformatted=$$(gofmt -l .) && \
	  if [ -n "$$unformatted" ]; then \
	    echo "❌ ERROR: The following files are not formatted with gofmt:"; \
	    echo "$$unformatted"; \
	    echo ""; \
	    echo "Run 'make format-go' or 'cd homeautomation-go && gofmt -w .' to fix"; \
	    exit 1; \
	  fi
	@echo "✅ gofmt formatting check passed"
	@echo ""
	@echo "📦 Step 2/7: Checking goimports formatting..."
	@cd homeautomation-go && \
	  if ! command -v goimports >/dev/null 2>&1; then \
	    echo "⚠️  goimports not installed. Installing..."; \
	    go install golang.org/x/tools/cmd/goimports@latest; \
	  fi && \
	  GOIMPORTS=$$(command -v goimports 2>/dev/null || echo "$(HOME)/go/bin/goimports") && \
	  unformatted=$$($$GOIMPORTS -l .) && \
	  if [ -n "$$unformatted" ]; then \
	    echo "❌ ERROR: The following files have import formatting issues:"; \
	    echo "$$unformatted"; \
	    echo ""; \
	    echo "Run 'cd homeautomation-go && goimports -w .' to fix"; \
	    exit 1; \
	  fi
	@echo "✅ goimports formatting check passed"
	@echo ""
	@echo "🔎 Step 3/7: Running go vet static analysis..."
	@cd homeautomation-go && go vet ./...
	@echo "✅ go vet passed"
	@echo ""
	@echo "🔬 Step 4/7: Running staticcheck linting..."
	@cd homeautomation-go && \
	  if ! command -v staticcheck >/dev/null 2>&1; then \
	    echo "⚠️  staticcheck not installed. Installing..."; \
	    go install honnef.co/go/tools/cmd/staticcheck@latest; \
	  fi && \
	  STATICCHECK=$$(command -v staticcheck 2>/dev/null || echo "$(HOME)/go/bin/staticcheck") && \
	  $$STATICCHECK ./...
	@echo "✅ staticcheck passed"
	@echo ""
	@echo "🔨 Step 5/7: Building all packages..."
	@cd homeautomation-go && go build ./...
	@echo "✅ Build successful"
	@echo ""
	@echo "🧪 Step 6/7: Running all tests..."
	@cd homeautomation-go && go test ./...
	@echo "✅ All tests passed"
	@echo ""
	@echo "🏁 Step 7/7: Running tests with race detector..."
	@cd homeautomation-go && go test -race ./...
	@echo "✅ Race detector passed (including integration tests)"
	@echo ""
	@echo "════════════════════════════════════════════════════════════════════"
	@echo "🎉 All pre-commit checks passed! Your code is ready to commit."
	@echo "════════════════════════════════════════════════════════════════════"

#format-go: @ Format Go code with gofmt and goimports
format-go:
	@echo "🎨 Formatting Go code..."
	@cd homeautomation-go && gofmt -w .
	@cd homeautomation-go && \
	  if ! command -v goimports >/dev/null 2>&1; then \
	    echo "⚠️  goimports not installed. Installing..."; \
	    go install golang.org/x/tools/cmd/goimports@latest; \
	  fi && \
	  (command -v goimports >/dev/null 2>&1 && goimports -w . || $(HOME)/go/bin/goimports -w .)
	@echo "✅ Code formatted successfully"

#lint-go: @ Run all Go linters (go vet, staticcheck)
lint-go:
	@echo "🔬 Running Go linters..."
	@cd homeautomation-go && go vet ./...
	@cd homeautomation-go && \
	  if ! command -v staticcheck >/dev/null 2>&1; then \
	    echo "⚠️  staticcheck not installed. Installing..."; \
	    go install honnef.co/go/tools/cmd/staticcheck@latest; \
	  fi && \
	  (command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || $(HOME)/go/bin/staticcheck ./...)
	@echo "✅ All linters passed"
