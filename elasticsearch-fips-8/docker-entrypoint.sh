#!/bin/bash
set -e

# Files created by Elasticsearch should always be group writable too
umask 0002

# Allow user specify custom CMD, maybe bin/elasticsearch itself
# for example to directly specify `-E` style parameters for elasticsearch on k8s
# or simply to run /bin/bash to check the image
if [[ "$1" == "eswrapper" || $(basename "$1") == "elasticsearch" ]]; then
  # Rewrite CMD args to remove the explicit command,
  # so that we are backwards compatible with the docs
  # from the previous Elasticsearch versions < 6
  # and configuration option:
  # https://www.elastic.co/guide/en/elasticsearch/reference/5.6/docker.html#_d_override_the_image_8217_s_default_ulink_url_https_docs_docker_com_engine_reference_run_cmd_default_command_or_options_cmd_ulink
  # Without this, user could specify `elasticsearch -E x.y=z` but
  # `bin/elasticsearch -E x.y=z` would not work. In any case,
  # we want to continue through this script, and not exec early.
  set -- "${@:2}"
else
  # Run whatever command the user wanted
  exec "$@"
fi

# Allow environment variables to be set by creating a file with the
# contents, and setting an environment variable with the suffix _FILE to
# point to it. This can be used to provide secrets to a container, without
# the values being specified explicitly when running the container.
#
# This is also sourced in elasticsearch-env, and is only needed here
# as well because we use ELASTIC_PASSWORD below. Sourcing this script
# is idempotent.
source /usr/share/elasticsearch/bin/elasticsearch-env-from-file

# Modified by Chainguard
# The KeyStore password must always be set when used in a FIPS environment
if [[ -f bin/elasticsearch-users ]]; then
  if [[ -n "$ELASTIC_PASSWORD" ]]; then
    # Check for existing KeyStore
    if ! [[ -f /usr/share/elasticsearch/config/elasticsearch.keystore ]]; then
      # If KeyStore password provided, set that as the password
      if [[ -n "$KEYSTORE_PASSWORD" ]]; then
        elasticsearch-keystore create -p <<EOF
$KEYSTORE_PASSWORD
$KEYSTORE_PASSWORD
EOF
      else
        # Exit when no KeyStore password has been provided
        echo "ERROR: No KeyStore password provided. Please set KEYSTORE_PASSWORD" && exit 1
      fi
    fi
  fi
fi

if [[ -n "$ES_LOG_STYLE" ]]; then
  case "$ES_LOG_STYLE" in
    console)
      # This is the default. Nothing to do.
      ;;
    file)
      # Overwrite the default config with the stack config. Do this as a
      # copy, not a move, in case the container is restarted.
      cp -f /usr/share/elasticsearch/config/log4j2.file.properties /usr/share/elasticsearch/config/log4j2.properties
      ;;
    *)
      echo "ERROR: ES_LOG_STYLE set to [$ES_LOG_STYLE]. Expected [console] or [file]" >&2
      exit 1 ;;
  esac
fi

if [[ -n "$ENROLLMENT_TOKEN" ]]; then
  POSITIONAL_PARAMETERS="--enrollment-token $ENROLLMENT_TOKEN"
else
  POSITIONAL_PARAMETERS=""
fi

# Signal forwarding and child reaping is handled by `tini`, which is the
# actual entrypoint of the container
exec /usr/share/elasticsearch/bin/elasticsearch "$@" $POSITIONAL_PARAMETERS <<<"$KEYSTORE_PASSWORD"
