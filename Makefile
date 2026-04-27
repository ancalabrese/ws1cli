BINARY   := ws1
BIN_DIR  := bin
SPEC     := spec.json
OAPI_CFG := oapi-codegen.yaml

.PHONY: all build install clean validate gen-client gen-cli gen

all: build


# spec validation
validate:
	go run github.com/getkin/kin-openapi/cmd/validate@latest -- $(SPEC)


# generate API client from spec
gen-client: validate
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
		--config $(OAPI_CFG) $(SPEC)

# generate cli bindings
gen-cli: validate
	go run ./cmd/gen_cli

gen: gen-client gen-cli

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/ws1

install:
	go install ./cmd/ws1

clean:
	rm -rf $(BIN_DIR)
