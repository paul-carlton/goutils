#!/bin/bash

# Utility to gofmt, goimports and test a package
# Version: 1.0

#!/usr/bin/env bash

# Utility setting local kubernetes cluster
# Version: 1.0
# Author: Paul Carlton (mailto:paul.carlton414@gmail.com)

set -euo pipefail

function usage()
{
    echo "usage ${0} [--debug] [--flux-bootstrap] [--flux-reset] [--no-wait] [--install]" >&2
    echo "This script will initialize docker kubernetes" >&2
    echo "  --debug: emmit debugging information" >&2
    echo "  --flux-bootstrap: force flux bootstrap" >&2
    echo "  --flux-reset: unistall flux before reinstall" >&2
    echo "  --cluster-type: the cluster type for Linux, k0s or kind, defaults to k0s" >&2
    echo "  --no-wait: do not wait for flux to be ready" >&2
    echo "  --install: install software required by kind cluster deployment" >&2
}

function args()
{
  wait=1
  install=""
  bootstrap=0
  reset=0
  debug_str=""
  cluster_type="k0s"
  arg_list=( "$@" )
  arg_count=${#arg_list[@]}
  arg_index=0
  while (( arg_index < arg_count )); do
    case "${arg_list[${arg_index}]}" in
          "--debug") set -x; debug_str="--debug";;
          "--no-wait") wait=0;;
          "--install") install="--install";;
          "--flux-bootstrap") bootstrap=1;;
          "--flux-reset") reset=1;;
          "--cluster-type") (( arg_index+=1 )); cluster_type="${arg_list[${arg_index}]}";;
               "-h") usage; exit;;
           "--help") usage; exit;;
               "-?") usage; exit;;
        *) if [ "${arg_list[${arg_index}]:0:2}" == "--" ];then
               echo "invalid argument: ${arg_list[${arg_index}]}" >&2
               usage; exit
           fi;
           break;;
    esac
    (( arg_index+=1 ))
  done
  
  if [ "$aws" == "true" ]; then
    if [ -z "$AWS_PROFILE" ]; then
      echo "AWS_PROFILE not set" >&2
      exit 1
    fi
  fi
}

args "$@"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
source $SCRIPT_DIR/envs.sh

if [ -z "$1" ] ; then
	echo "please specify a package"
	exit 1
fi

package_name="$1"

if [ ! -d "$package_name" ] ; then
	echo "package: $package_name does not exist"
	exit 1
fi

TOP=`git rev-parse --show-toplevel`
gmake -C $TOP/$package_name --makefile=$TOP/makefile.mk gofmt gofumpt
gmake -C $TOP/$package_name --makefile=$TOP/makefile.mk clean
gmake -C $TOP/$package_name --makefile=$TOP/makefile.mk
