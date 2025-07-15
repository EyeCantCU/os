import unittest
import os

from medusa.storage.google_storage import GoogleStorage
from medusa.config import *
from medusa.config import _build_default_config, _namedtuple_from_dict

BUCKET_NAME_ENV = "C3_TEST_BUCKET"
KEY_FILE_ENV = "C3_GCP_SA_KEY_FILE"


class GCSTest(unittest.TestCase):

    def test_gcs_list(self):
        bucket_name = os.environ[BUCKET_NAME_ENV]
        key_file = os.environ[KEY_FILE_ENV]

        config = _build_default_config()

        config.set('storage', 'bucket_name', bucket_name)
        config.set('storage', 'key_file', key_file)

        gcs = GoogleStorage(_namedtuple_from_dict(StorageConfig, config['storage']))

        gcs.connect()

        blobs_list = None
        has_thrown = False

        # We expect GoogleStorage.list_blobs() to not crash.

        try:
            blobs_list = gcs.list_blobs()
        except:
            has_thrown = True

        assert not has_thrown
        assert len(blobs_list) > 0
