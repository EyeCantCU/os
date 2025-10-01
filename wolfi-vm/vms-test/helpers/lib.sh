#!/usr/bin/env bash

stderr() { echo "$@" 1>&2; }
failrc() { local rc=$1; shift; stderr "$@"; exit "$rc"; }
fail() { local r=$?;  [ "$r" -eq 0 ] && r=1; failrc "$r" "$@"; }

log() {
    local now=""
    now=$(date '+%Y/%m/%d %H:%M:%S')
    [ -z "$LOG" ] || echo "$now" "$@" >> "$LOG"
    stderr "$now" "$@"
}

# vr(cmd) - run cmd, log execution and result. stdout and stderr unmodified.
vr() {
    local rc="" sstart=$SECONDS
    log "execute: $*"
    "$@"
    rc=$?
    log ":: exit $rc [$((SECONDS-sstart))s]"
    return $rc
}

# vrl(output, cmd) - verbose run log.
#   run cmd with output to output.stdout and output.stderr
vrl() {
    local output="$1" rc="" sstart=$SECONDS
    shift
    log "execute: $*"
    "$@" >"$output.stdout" 2>"$output.stderr"
    rc=$?
    log ":: exit $rc [$((SECONDS-sstart))s]"
    return "$rc"
}

# rq(cmd) - run cmd quietly unless it fails
rq() {
    local rc="" out=""
    out=$("$@" 2>&1) && return 0
    rc=$?
    stderr "failed [$rc]: $*"
    printf "%s\n" "$out" | sed -e 's,^,> ,' 1>&2
    return "$rc"
}

dump_awspubin() {
    local f="$1" out="" arch="" localname=""
    out=$(jq -r '.images | to_entries[] | . as $entry | $entry.value |
        to_entries[] | "\(.key) \(.value) \($entry.key)"' < "$f")
    # shellcheck disable=SC2030
    localname="${f##*/}"
    localname="${localname/.json/}"
    echo "$out" |
        while read -r region ami name; do
            # 'name' looks like chainguard-docker-x86_64-20250501-1545
            case "$name" in
                *-arm64-*)
                    arch=aarch64
                    family=${name%-arm64-*}
                    ;;
                *-x86_64-*)
                    arch=x86_64
                    family=${name%-x86_64-*}
                    ;;
                *) stderr "could not find arch in $name"; return 1;;
            esac
            echo "$region $ami $arch $family $localname"
        done
}

parse_awspubins() {
    local i="" f="" n=0
    for i in "$@"; do
        if [ -d "$i" ]; then
            for f in "$i"/*.json; do
                dump_awspubin "$f" || { stderr "failed reading $f in $i"; return 1; }
                n=$((n+1))
            done
        elif [ -f "$i" ]; then
            dump_awspubin "$i" || { stderr "failed reading $f"; return 1; }
            n=$((n+1))
        else
            stderr "$f: not a file"
            return 1
        fi
    done
    [ "$n" -ne 0 ] || { stderr "did not process any files"; return 1; }
    return 0
}
