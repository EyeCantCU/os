#!/usr/bin/env bash

set -o pipefail
WAIT_SSH_TIMEOUT=${WAIT_SSH_TIMEOUT:-5m}

failrc() { local rc=$?; stderr "$@"; exit "$rc"; }
stderr() { echo "$@" 1>&2; }

# vr(cmd) - run cmd, log execution and result. stdout and stderr unmodified.
vr() {
    local rc="" sstart=$SECONDS
    log "execute: $*"
    "$@"
    rc=$?
    log ":: exit $rc [$((SECONDS-sstart))s]"
    return $rc
}

log() {
    local now=""
    now=$(date '+%Y/%m/%d %H:%M:%S')
    echo "$now" "$@"
}

# findarg(--arg, "$@")
findarg() {
    local flag="$1"
    shift
    while [ $# -ne 0 ]; do
        { cur="$1"; next="$2"; }
        case "$cur" in
            "$flag") printf "%s" "$next"; return 0;;
            "$flag"=*) printf "%s" "${cur#*=}"; return 0;;
            --) shift; break;;
        esac
        shift;
    done
    return 1
}

TIMEOUT_PID=""
timeout() {
    local own_pid="$$"
    { sleep "$WAIT_SSH_TIMEOUT"; kill "$own_pid" 2>/dev/null && log "$@"; } &
    TIMEOUT_PID="$!"
}

cancel_timeout() {
    [ -z "$TIMEOUT_PID" ] && return 0
    TIMEOUT_PID=""
    kill "$TIMEOUT_PID" 2>/dev/null
}

main() {
    local region="" tag="" instance_id=""
    region=$(findarg --region "$@") || failrc "can't find --region"
    tag=$(findarg --tag "$@") || failrc "can't find --tag"

    # Launch instance
    vr "$RUNNER" launch "$@" || failrc "can't launch instance"

    # Wait for instance readiness
    timeout "timeout waiting for ssm agent node to appear"
    trap cancel_timeout exit
    vr "$RUNNER" wait-for-ssh --timeout "$WAIT_SSH_TIMEOUT" --region "$region" --tag "$tag" \
        || failrc "ssh never became available"

    # Wait for SSM agent registration
    log "waiting for ssm agent to register with control plane"
    instance_id=$(
        aws ec2 describe-instances --region "$region" --filters "Name=tag:Name,Values=$tag" \
            | jq -re '.Reservations[0].Instances[0].InstanceId'
    ) || failrc "can't find instance ID"

    local nodes="0"
    while [ "$nodes" == "0" ]; do
        nodes=$(
            aws ssm list-nodes \
                    --region "$region" \
                    --filters "Key=InstanceId,Values=$instance_id" \
                              "Key=InstanceStatus,Values=Active" \
                              "Key=ManagedStatus,Values=Managed" \
                | jq -re '.Nodes | length'
        ) || failrc "can't list ssm agent nodes"
        sleep 5
    done

    # Wait for instance to be known online
    local ping_status=
    while [[ "$ping_status" != 'Online' ]]; do
        ping_status=$(
            aws ssm describe-instance-information \
                 --region "$region" \
                 --instance-information-filter-list "key=InstanceIds,valueSet=$instance_id" \
                 --query 'InstanceInformationList[0].PingStatus' \
                 --output text
        )
        sleep 5
    done

    # Send command
    local command_id=
    command_id=$(
        aws ssm send-command --region "$region" \
            --document-name "AWS-RunShellScript" \
            --parameters 'commands=["printf '\''hello wolfi'\'' > /tmp/hello-wolfi.txt"]' \
            --targets "Key=tag:Name,Values=$tag" \
            --comment "wolfi-vm-test-hello-wolfi" \
            --query 'Command.CommandId' \
            --output text
    ) || failrc "cannot send command to $instance_id ($tag)"

    # Wait for command to execute
    cancel_timeout
    timeout 'timeout waiting for ssm agent to execute the command'
    vr aws ssm wait command-executed --region "$region" --instance-id "$instance_id" --command-id "$command_id"
}

main "$@"
