BUILD_ARMV6 := GOOS=linux GOARCH=arm GOARM=6 go build
BUILD_ARMV7 := GOOS=linux GOARCH=arm GOARM=7 go build

OUT_DIR := out

all: iotcorelogger readtemp

.PHONY: iotcorelogger
iotcorelogger: proto
	go build -o $(OUT_DIR)/$@ ./cmd/$@
	$(BUILD_ARMV7) -o $(OUT_DIR)/armv7/$@ ./cmd/$@
	$(BUILD_ARMV6) -o $(OUT_DIR)/armv6/$@ ./cmd/$@

.PHONY: readtemp
readtemp:
	go build -o $(OUT_DIR)/$@ ./cmd/$@
	$(BUILD_ARMV7) -o $(OUT_DIR)/armv7/$@ ./cmd/$@
	$(BUILD_ARMV6) -o $(OUT_DIR)/armv6/$@ ./cmd/$@

.PHONY: proto
proto:
	protoc --go_out=measurement --python_out=client_python/loggers measurement.proto

clean:
	rm -rf $(OUT_DIR)
