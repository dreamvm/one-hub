import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import export_binary


class BinaryEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.binary = self.root / "candidate"
        self.info = self.root / "build-info.txt"
        # The exporter tests bytes/provenance; buildinfo's Go tests verify actual executables.
        self.binary.write_bytes(b"synthetic program bytes")
        self.sha = "1" * 40
        self.image = "sha256:" + "2" * 64
        self.output = self.root / "evidence"
        digest = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        self.info.write_text(
            "CANDIDATE_BUILD_INFO Go=go1.25.14 main=one-api GOOS=linux "
            f"GOARCH=amd64 CGO_ENABLED=1 SHA256={digest}\n"
        )

    def export(self, sha=None):
        return export_binary.export(self.binary, self.info, sha or self.sha, self.image, self.output)

    def test_keeps_exact_bytes_and_source_identity(self):
        record = self.export()
        self.assertEqual(self.binary.read_bytes(), (self.output / "one-api").read_bytes())
        self.assertEqual(record, json.loads((self.output / "manifest.json").read_text()))
        self.assertEqual(self.sha, record["source_sha"])
        self.assertEqual(self.image, record["image_id"])
        self.assertEqual("amd64", record["goarch"])
        self.assertEqual({"one-api", "manifest.json"}, {p.name for p in self.output.iterdir()})

    def test_changed_program_is_not_retained(self):
        self.binary.write_bytes(b"different synthetic program")
        with self.assertRaisesRegex(ValueError, "bytes differ"):
            self.export()
        self.assertFalse(self.output.exists())

    def test_existing_output_is_preserved(self):
        self.output.mkdir()
        marker = self.output / "existing"
        marker.write_text("keep")
        with self.assertRaises(FileExistsError):
            self.export()
        self.assertEqual("keep", marker.read_text())

    def test_ref_or_unverified_identity_does_not_create_output(self):
        with self.assertRaisesRegex(ValueError, "exact commit"):
            self.export("main")
        self.info.write_text(self.info.read_text().replace("go1.25.14", "go1.25.0"))
        with self.assertRaisesRegex(ValueError, "build identity"):
            self.export()
        self.assertFalse(self.output.exists())

    def test_binary_size_is_bounded_before_copying(self):
        with patch.object(export_binary, "MAX_BINARY_BYTES", 1):
            with self.assertRaisesRegex(ValueError, "size bound"):
                self.export()
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
