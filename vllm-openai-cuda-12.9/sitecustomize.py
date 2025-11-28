# sitecustomize.py - FIPS-safe shim for hashlib.md5
# vLLM uses MD5 for non-cryptographic purposes (file hashing for caching).
# Under FIPS mode, these calls fail even though they're not security-related.
# This shim defaults usedforsecurity=False for callers that don't specify it.

import hashlib as _h
_real_md5 = _h.md5


def _md5_fips_ok(*args, **kwargs):
    kwargs.setdefault("usedforsecurity", False)
    return _real_md5(*args, **kwargs)


_h.md5 = _md5_fips_ok
