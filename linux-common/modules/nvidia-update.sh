#!/bin/bash
set -o pipefail

VERBOSE=0

Usage() {
    cat <<EOF
Usage: ${0##*/}

  List released versions of an nvidia tesla driver.

           --version     VERSION   Major version to check (required)
           --latest                Report only latest version
           -v|--verbose
EOF
}

stderr() { echo "$@" 1>&2; }

vlog() {
    [ $VERBOSE -gt 0 ] || return 0
    stderr "$@"
}

main(){
    local short_opts="h,v"
    local long_opts="help,verbose,latest,version:"
    local getopt_out=""
    # shellcheck disable=SC2015
    getopt_out=$(getopt --name "${0##*/}" \
        --options "${short_opts}" --long "${long_opts}" -- "$@") &&
        eval set -- "${getopt_out}" ||
        { Usage 1>&2; return 1; }

    local version="" latest=false

    while [ $# -ne 0 ]; do
        { cur="$1"; next="$2"; }
        case "$cur" in
            -h|--help) Usage ; exit 0;;
               --version) version="$next"; shift;;
               --latest) latest=true; shift;;
            -v|--verbose) VERBOSE=1;;
            --) shift; break;;
        esac
        shift;
    done
    [ -z "$version" ] && { stderr "--version is required"; Usage 1>&2; return 1; }
    local release_url="https://docs.nvidia.com/datacenter/tesla/drivers/releases.json"
    vlog "release url: ${release_url}"
    vlog "looking for version: ${version}"
    # Find versions,
    versions=$(curl -sSL "$release_url") || {
        stderr "couldn't fetch release info"
        return 1
    }
    newest=$(echo "$versions" | jq -re ".[\"$version\"]") || {
        stderr "couldn't get newest package version"
        return 1
    }
    vlog "version json for $version: $newest"
    if [ "$latest" = "true" ]; then
      echo "$newest" | jq -re '.driver_info[0].release_version'
    else
      echo "$newest" | jq -re '.driver_info[].release_version'
    fi
}
main "$@"
