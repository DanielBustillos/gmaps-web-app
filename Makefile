BINARY_NAME := mapsscrap

# Default binary name
BINARY_NAME=mapsscrap

# Build the application
build:
	go build -o bin/$(BINARY_NAME) main.go

# Build the mapsscrap-1 binary specifically
build-mapsscrap:
	go build -o mapsscrap-1 main.go

# Build phone scraper
build-phone:
	go build -o phone_scraper enhanced_phone_scraper.go

# Build web server
build-web:
	go build -o web_server web_server.go

# Build all binaries
build-all: build-mapsscrap build-phone build-web

# Start web interface
web:
	./start_web.sh

# Run the pipeline with sample parameters
run-pipeline:
	./pipeline.sh 19.1019061 -98.2810447 "spa" 2.0

# Run the application with sample parameters for Mexico City
run:
	go run main.go --lat 19.4343491 --lon -99.1775742 --query "dentist" --radius 2

# Test phone scraper with single URL
test-phone:
	go run enhanced_phone_scraper.go url --url "https://www.google.com/maps/place/Model+Art+Spa+Plaza+Aure/data=!4m7!3m6!1s0x85cfc5ec051634e9:0x4f65d92bbc9f0dae!8m2!3d19.1019061!4d-98.2810447!16s%2Fg%2F11xfjlpwmb!19sChIJ6TQWBezFz4URrg2fvCvZZU8?authuser=0&hl=es-419&rclk=1"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f $(BINARY_NAME) mapsscrap-1 phone_scraper

# Tidy and verify dependencies
tidy:
	go mod tidy
	go mod verify

# Run tests
test:
	go test ./...

# Install dependencies
deps:
	go get -v ./...

# Format code
fmt:
	go fmt ./...