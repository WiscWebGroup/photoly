import hashlib
import importlib.util
import pathlib
import tempfile
import unittest

path = pathlib.Path(__file__).with_name('collect-debian-sources.py')
spec = importlib.util.spec_from_file_location('collector', path)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class SourceVerification(unittest.TestCase):
    def test_rejects_corrupt_or_missing_source(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            data = b'original upstream source'
            (root / 'source.tar.gz').write_bytes(data)
            (root / 'source.dsc').write_text('Checksums-Sha256:\n %s %d source.tar.gz\n' % (hashlib.sha256(data).hexdigest(), len(data)))
            collector.verify(root)
            (root / 'source.tar.gz').write_bytes(b'replacement')
            with self.assertRaisesRegex(RuntimeError, 'checksum mismatch'):
                collector.verify(root)
            (root / 'source.tar.gz').unlink()
            with self.assertRaises(FileNotFoundError):
                collector.verify(root)

    def test_rejects_paths_outside_source_directory(self):
        with self.assertRaisesRegex(RuntimeError, 'Invalid source filename'):
            collector.checksums('Checksums-Sha256:\n deadbeef 10 ../outside\n')


if __name__ == '__main__':
    unittest.main()
