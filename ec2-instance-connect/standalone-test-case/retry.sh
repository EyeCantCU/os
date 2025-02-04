#!/bin/sh
set -e
rm -f $(pwd)/eic-YsCwlHHE/*.pem
rm -rf $(pwd)/eic-YsCwlHHE/eic-cert-*

# The below were extracted/intercepted from a live EC2 instance within
# the short time window when keys are published. As published keys are
# signed and revoked, which are verified by the parser use faketime to
# get back into validity window. The upstream test-suite that should
# generate CA, intermediate CA, oscap revocation chains, and validate
# everything appears to have bitrotted as certs it generates lack
# required x509 critical & exteneded attributes and when fixing that
# all test cases fail.
extracted_key=$(faketime @1738233605 \
eic_parse_authorized_keys \
 -x true -p eic-YsCwlHHE/eic-keys -o /usr/bin/openssl -d $(pwd)/eic-YsCwlHHE \
 -s "$(cat certificate.var)" -i i-076d41e325c94200c -c managed-ssh-signer.us-east-1.amazonaws.com -a /etc/ssl/certs/ca-bundle.crt \
 -v $(pwd)/eic-YsCwlHHE/eic-ocsp-5CVaeTe0)
rm -f $(pwd)/eic-YsCwlHHE/*.pem

if [ "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHQmprB1y9JdpYxagDOL2Alliiwbf14LLhz8W9m4XgOl" != "$extracted_key" ]; then
    echo "I am sorry instance connect did not work :-("
    echo "Good luck debugging it"
    exit 1
else
    echo "Key successfully retrieved"
fi



