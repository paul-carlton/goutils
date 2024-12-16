# Developers Guide

This repository contains testing and development utilities.

## Setup

clone into $GOPATH/src/github.com/nabancard/goutils:

    mkdir -p $GOPATH/src/github.com/nabancard
    cd $GOPATH/src/github.com/nabancard
    git clone git@github.com:nabancard/goutils.git
    cd goutils

This project requires the following software:

    golangci-lint --version = 1.30.0
    golang version >= 1.13.1

You can install these in the project bin directory using the 'setup.sh' script:

    bin/setup.sh

The setup.sh script can safely be run at any time. It installs the required software.

## Development

The Makefile in the project's top level directory will compile, build and test all components.

    make

To run the build and test in a docker container, type:

    make check

Each package directory contains a go.mod file so that users can import the desired version of the package. When making changes tag the commit with the next semantic version for that package, e.g.

    git commit -a -m "details of changes"
    git push
    git tag pkg/logging/v0.0.0
    git push --tags

Users of the 'logging' package can then update their go.mod file to reference the new version.

To contribute create a branch and submit a pull request.
