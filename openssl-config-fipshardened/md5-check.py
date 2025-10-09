import hashlib

print(hashlib.md5("test_str".encode('utf-8'), usedforsecurity=False).hexdigest())
