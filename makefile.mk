# Makes a recipe passed to a single invocation of the shell.
.ONESHELL:

MAKEFILE_PATH:=$(abspath $(dir $(lastword $(MAKEFILE_LIST))))

GO_SOURCES:=$(wildcard *.go)
GO_TEST_SOURCES:=$(wildcard *test.go)
BASH_SOURCES:=$(wildcard *.sh)
COVERAGE_DIR:=$(CURDIR)/coverage
COVERAGE_HTML_DIR:=$(COVERAGE_DIR)/html

COVERAGE_ARTIFACT:=${COVERAGE_HTML_DIR}/main.html
LINT_ARTIFACT:=._gometalinter
TEST_ARTIFACT:=${COVERAGE_DIR}/coverage.out
BASH_ARTIFACT:=._shellcheck
GOMOD_ARTIFACT:=._gomod

YELLOW:=\033[0;33m
GREEN:=\033[0;32m
RED:=\033[0;31m
NC:=\033[0m
NC_DIR:=: $(CURDIR)$(NC)

.PHONY: all clean goimports gofumpt gofmt clean-lint lint clean-test test \
	clean-gomod gomod gomod-update go-generate \
	clean-coverage coverage
# Stop prints each line of the recipe.
.SILENT:

all: gomod-update lint coverage go-generate
clean: clean-lint clean-coverage clean-test clean-gomod

clean-gomod:
	rm -rf ${GOMOD_ARTIFACT}

goimports: ${GO_SOURCES}
	echo "${YELLOW}Running goimports${NC_DIR}" && \
	goimports -w $^


gofmt: ${GO_SOURCES}
	echo "${YELLOW}Running gofmt${NC_DIR}" && \
	gofmt -w -s $^

gofumpt: ${GO_SOURCES}
	echo "${YELLOW}Running gofumpt${NC_DIR}" && \
	gofumports -w $^

clean-test:
	rm -rf $(dir ${TEST_ARTIFACT})

test: ${TEST_ARTIFACT}
${TEST_ARTIFACT}: ${GO_SOURCES}
	if [ -n "${GO_TEST_SOURCES}" ]; then
		{ echo "${YELLOW}Running go test${NC_DIR}" && \
		  mkdir -p $(dir ${TEST_ARTIFACT}) && \
		  go test -coverprofile=$@ -v && \
		  echo "${GREEN}TEST PASSED${NC}"; } || \
		{ $(MAKE) --makefile=$(lastword $(MAKEFILE_LIST)) clean-test && \
          echo "${RED}TEST FAILED${NC}" && \
		  exit 1; }
	fi

clean-coverage: clean-test
	rm -rf $(dir ${COVERAGE_ARTIFACT})

coverage: ${COVERAGE_ARTIFACT}
${COVERAGE_ARTIFACT}: ${TEST_ARTIFACT}
	if [ -e "$<" ]; then
		echo "${YELLOW}Running go tool cover${NC_DIR}" && \
		mkdir -p $(dir ${COVERAGE_ARTIFACT}) && \
		go tool cover -html=$< -o $@ && \
		echo "${GREEN}Generated: $@${NC}"
	fi


clean-lint:
	rm -f ${LINT_ARTIFACT}

lint: ${LINT_ARTIFACT}
${LINT_ARTIFACT}: ${MAKEFILE_PATH}/golangci-lint.yml ${GO_SOURCES}
	echo "${YELLOW}Running go lint${NC_DIR}" && \
	(cd $(CURDIR) && \
	 procs=$$(expr $$( \
		(grep -c ^processor /proc/cpuinfo || \
		 sysctl -n hw.ncpu || \
		 echo 1) 2>/dev/null) '*' 2 '-' 1) && \
	GOPROXY=https://proxy.golang.org,direct \
	golangci-lint run \
		--config ${MAKEFILE_PATH}/golangci-lint.yml \
		--concurrency=$${procs} \
		--path-prefix "$$(realpath --relative-to ${MAKEFILE_PATH} ${CURDIR})/." \
		.) && \
	touch $@ && \
	cd ${MAKEFILE_PATH}

go.mod:
	go mod tidy

go-generate:
	go generate ./...

gomod: go.sum
go.sum:  ${GOMOD_ARTIFACT}
%._gomod: go.mod
	touch  ${GOMOD_ARTIFACT}

${GOMOD_ARTIFACT}: gomod-update
gomod-update: go.mod ${PROJECT_SOURCES}
	echo "${YELLOW}go build $(CURDIR) ${NC}" && \
	go build ./... && \
	echo "${YELLOW}go mod tidy${NC}" && \
	go mod tidy && \
	echo "${YELLOW}go mod download${NC}" && \
	go mod download