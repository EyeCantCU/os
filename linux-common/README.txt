These base configs are loosely based on the Debian cloud kernel configs with excess things pulled out.

This structure removes system level config snippets in favor of a generic base for each arch and environment specific config (per cloud).

```
# Generic common configs (should support KVM on qemu)
config-generic-x86
config-generic-arm

# Cloud specific additions
config-aws-generic
config-azure-generic
config-gcp-generic
config-qemu-generic
```
