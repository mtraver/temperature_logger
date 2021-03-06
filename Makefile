BUILD := go build
BUILD_ARMV6 := GOOS=linux GOARCH=arm GOARM=6 $(BUILD) -ldflags="-s -w"
BUILD_ARMV7 := GOOS=linux GOARCH=arm GOARM=7 $(BUILD) -ldflags="-s -w"

OUT_DIR := out

all: iotcorelogger readtemp api apiclient

.PHONY: iotcorelogger
iotcorelogger: proto
	$(BUILD) -o $(OUT_DIR)/$@ ./cmd/$@
	$(BUILD_ARMV7) -o $(OUT_DIR)/armv7/$@ ./cmd/$@
	$(BUILD_ARMV6) -o $(OUT_DIR)/armv6/$@ ./cmd/$@

.PHONY: readtemp
readtemp:
	$(BUILD) -o $(OUT_DIR)/$@ ./cmd/$@
	$(BUILD_ARMV7) -o $(OUT_DIR)/armv7/$@ ./cmd/$@
	$(BUILD_ARMV6) -o $(OUT_DIR)/armv6/$@ ./cmd/$@

.PHONY: api
api:
	$(BUILD) -o $(OUT_DIR)/$@ ./cmd/$@

.PHONY: apiclient
apiclient:
	$(BUILD) -o $(OUT_DIR)/$@ ./cmd/$@

.PHONY: proto
proto:
	# Get protoc from https://github.com/protocolbuffers/protobuf/releases
	# Install the Go and Go gRPC plugins like this:
	#   go install google.golang.org/protobuf/cmd/protoc-gen-go
	#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	protoc --include_imports --include_source_info \
	  --descriptor_set_out=measurementpb/measurement.pb.descriptor \
	  --go_out=module=github.com/mtraver/environmental-sensor:. \
	  --go-grpc_out=module=github.com/mtraver/environmental-sensor:. \
	  measurement.proto

	protoc --go_out=module=github.com/mtraver/environmental-sensor:. \
	  configpb/config.proto

clean:
	rm -rf $(OUT_DIR)
