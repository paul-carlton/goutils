# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.DEFAULT_GOAL := all

include project-name.mk

# Makes a recipe passed to a single invocation of the shell.
.ONESHELL:

MAKE_SOURCES:=makefile.mk project-name.mk Makefile
PROJECT_SOURCES:=$(shell find ./pkg/ -regex '.*.\.\(go\|json\)$$')

BUILD_DIR:=build/
export DOCKER_BUILD_PROXYS?="--build-arg HTTP_PROXY=${HTTP_PROXY} --build-arg HTTPS_PROXY=${HTTPS_PROXY}"

ALL_GO_PACKAGES:=$(shell find ${CURDIR}/pkg \
	-type f -name \*.go -exec dirname {} \; | sort --uniq)
GO_CHECK_PACKAGES:=$(shell echo $(subst $() $(),\\n,$(ALL_GO_PACKAGES)) | \
	awk '{print $$0}')

CHECK_ARTIFACT:=${BUILD_DIR}${PROJECT}-check-${VERSION}-docker.tar
BUILD_ARTIFACT:=${BUILD_DIR}${PROJECT}-build-${VERSION}-docker.tar
DEV_BUILD_ARTIFACT:=${BUILD_DIR}${PROJECT}-dev-build-${VERSION}-docker.tar

GO_BIN_ARTIFACT:=${GOBIN}/${PROJECT}

YELLOW:=\033[0;33m
GREEN:=\033[0;32m
NC:=\033[0m

# Targets that do not represent filenames need to be registered as phony or
# Make won't always rebuild them.
.PHONY: all clean clean-godocs go-generate \
	clean-gomod gomod gomod-update \
	clean-${PROJECT}-check ${PROJECT}-check ${GO_CHECK_PACKAGES} clean-check check 
# Stop prints each line of the recipe.
.SILENT:

# Allow secondary expansion of explicit rules.
.SECONDEXPANSION: %.md %-docker.tar

all: ${PROJECT}-check go-generate
clean: clean-gomod clean-${PROJECT}-check \
	clean-check clean-${BUILD_DIR}

clean-${BUILD_DIR}:
	rm -rf ${BUILD_DIR}

${BUILD_DIR}:
	mkdir -p $@

clean-${PROJECT}-check:
	$(foreach target,${GO_CHECK_PACKAGES},
		$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk clean;)

${PROJECT}-check: ${GO_CHECK_PACKAGES}
	$(foreach target,${GO_CHECK_PACKAGES},$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk;)

clean-gomod:
	$(foreach target,${GO_CHECK_PACKAGES},
		$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk clean-gomod;)

gomod-update:
	$(foreach target,${GO_CHECK_PACKAGES},
		$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk gomod-update;)

gomod:
	$(foreach target,${GO_CHECK_PACKAGES},
		$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk gomod;)

# Generate code
go-generate: ${GO_CHECK_PACKAGES}
	$(foreach target,${GO_CHECK_PACKAGES},
		$(MAKE) -C ${target} --makefile=${CURDIR}/makefile.mk go-generate;)

clean-${PROJECT}-build:
	rm -f ${GO_BIN_ARTIFACT}

clean-check:
	rm -f ${CHECK_ARTIFACT}

check: DOCKER_SOURCES=Dockerfile ${MAKE_SOURCES} ${PROJECT_SOURCES}
check: DOCKER_BUILD_OPTIONS=--target builder --no-cache
check: IMG=${PROJECT}-check:latest
check: ${BUILD_DIR} ${CHECK_ARTIFACT}

%-docker.tar: $${DOCKER_SOURCES}
	docker build --rm --pull=true \
		"${DOCKER_BUILD_PROXYS}" \
		${DOCKER_BUILD_OPTIONS} \
		--tag ${IMG} \
		--file $< \
		. && \
	docker save --output $@ ${IMG}

