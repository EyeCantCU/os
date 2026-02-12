import _hashlib
import hashlib
import hmac
import unittest

TEST_STRING = b"Chainguard"
EXPECTED_MD5 = "209915e31b55e2b0042f4967a017ac7d"

# This is a little gross, but old Python doesn't have the _hashlib exception.
try:
    EXPECTED_EXCEPTIONS = (ValueError, _hashlib.UnsupportedDigestmodError)
except AttributeError:
    EXPECTED_EXCEPTIONS = (ValueError,)


class MD5FipsTests(unittest.TestCase):
    def test_not_usedforsecurity(self):
        digest = hashlib.md5(TEST_STRING, usedforsecurity=False).hexdigest()
        assert digest == EXPECTED_MD5

    def test_usedforsecurity(self):
        with self.assertRaises(EXPECTED_EXCEPTIONS):
            _ = hashlib.md5(TEST_STRING, usedforsecurity=True).hexdigest()

    def test_hmac(self):
        with self.assertRaises(EXPECTED_EXCEPTIONS):
            _ = hmac.new(TEST_STRING, TEST_STRING, "md5")


if __name__ == "__main__":
    unittest.main()
