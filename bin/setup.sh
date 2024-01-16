#!/usr/bin/env bash
# Set versions of software required
linter_version=1.55.2
mockgen_version=v0.4.0

function usage()
{
    echo "USAGE: ${0##*/}"
    echo "Install software required for golang project"
}

function args() {
    while [ $# -gt 0 ]
    do
        case "$1" in
            "--help") usage; exit;;
            "-?") usage; exit;;
            *) usage; exit;;
        esac
    done
}

function install_linter() {
    TARGET=$(go env GOPATH)
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b "${TARGET}/bin" v${linter_version}
}

args "${@}"

sudo -E env >/dev/null 2>&1
if [ $? -eq 0 ]; then
    sudo="sudo -E"
fi

echo "Running setup script to setup software"
GO111MODULE=auto go install mvdan.cc/gofumpt@latest


golangci-lint --version 2>&1 | grep $linter_version >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing linter version: ${linter_version}"
    install_linter
    golangci-lint --version 2>&1 | grep $linter_version >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install linter"
        exit
    fi
else
    echo "linter version: `golangci-lint --version`"
fi

mockgen -version 2>&1 | grep ${mockgen_version} >/dev/null
ret_code="${?}"
if [[ "${ret_code}" != "0" ]] ; then
    echo "installing mockgen version: ${mockgen_version}"
    go install go.uber.org/mock/mockgen@${mockgen_version}
    mockgen -version 2>&1 | grep ${mockgen_version} >/dev/null
    ret_code="${?}"
    if [ "${ret_code}" != "0" ] ; then
        echo "Failed to install mockgen"
        exit
    fi
else
    echo "mockgen version: `mockgen -version`"
fi
