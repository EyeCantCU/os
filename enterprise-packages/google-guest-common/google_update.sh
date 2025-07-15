#!/bin/bash
set -o pipefail

VERBOSE=0

Usage() {
    cat <<EOF
Usage: ${0##*/}

  List latest versions of a package released to Debian in a repo run by Google.

           --latest              only show the latest version
           --package     PACKAGE   Package to check for (required)
           --wolfi-name  PACKAGE   Package to check for (default: --package)
           --release     RELEASE   Debian release codename to use (default: 'bookworm')
           --repo        REPO      Repository to check (default: 'google-compute-engine')
           --arch        ARCH      Architecture to check (default: 'amd64')
           -v|--verbose

  Note that this script is written to fetch multiple versions, but some teams at Google delete the previous package version when releasing a new one, so often there is only one version available.

EOF
}

stderr() { echo "$@" 1>&2; }

vlog() {
    [ $VERBOSE -gt 0 ] || return 0
    stderr "$@"
}

main(){
    local short_opts="h,v"
    local long_opts="help,latest,verbose,package:,release:,repo:,arch:"
    local getopt_out=""
    # shellcheck disable=SC2015
    getopt_out=$(getopt --name "${0##*/}" \
        --options "${short_opts}" --long "${long_opts}" -- "$@") &&
        eval set -- "${getopt_out}" ||
        { Usage 1>&2; return 1; }

    local latest=1 package_name="" debian_release="bookworm" gce_repository="google-compute-engine" arch="amd64"

    while [ $# -ne 0 ]; do
        { cur="$1"; next="$2"; }
        case "$cur" in
            -h|--help) Usage ; exit 0;;
               --package) package_name="$next"; shift;;
               --release) debian_release="$next"; shift;;
               --repo) gce_repository="$next"; shift;;
               --arch) arch="$next"; shift;;
               --latest) latest=0;;
            -v|--verbose) VERBOSE=1;;
            --) shift; break;;
        esac
        shift;
    done
    [ -z "$package_name" ] && { stderr "--package is required"; Usage 1>&2; return 1; }
    local apt_manifest_url="https://packages.cloud.google.com/apt/dists/${gce_repository}-${debian_release}-stable/main/binary-${arch}/Packages"
    vlog "Manifest URL: ${apt_manifest_url}"
    vlog "Looking for package: ${package_name}"
    local versions="" newest=""
    # Find versions,
    versions=$(curl -sSL "$apt_manifest_url" | grep -x -A 1 "Package: $package_name" | grep -oE "[0-9]{8}\.[0-9]{2}") || {
        vlog "couldn't fetch or parse package versions"
        return 1
    }
    newest=$(echo "$versions" | sort | head -n 1) || {
        vlog "couldn't get newest package version"
        return 1
    }
    if [ $latest -eq 0 ]; then
        echo "$newest"
    else
        echo "$versions"
    fi
}
main "$@"
