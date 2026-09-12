import hashlib
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

from upgrade import fixture_password, fixture_target, restore_database, validate_images


class UpgradeSafetyTests(unittest.TestCase):
    def test_passwords_fit_both_versions_validation(self):
        password = fixture_password()
        self.assertGreaterEqual(len(password), 8)
        self.assertLessEqual(len(password), 20)

    def args(self):
        return SimpleNamespace(old_image="sha256:" + "a" * 64, candidate_image="sha256:" + "b" * 64,
                               mysql_image="sha256:" + "c" * 64, redis_image="sha256:" + "d" * 64)

    def test_images_must_be_different_immutable_ids(self):
        validate_images(self.args())
        for field in ("old_image", "candidate_image", "mysql_image", "redis_image"):
            args = self.args()
            setattr(args, field, "latest")
            with self.assertRaises(AssertionError):
                validate_images(args)
        args = self.args()
        args.candidate_image = args.old_image
        with self.assertRaises(AssertionError):
            validate_images(args)

    def test_restore_rejects_unowned_or_production_named_databases(self):
        for name, created in (("mysql", [("container", "mysql")]), ("onehub-upgrade-aaaaaaaaaaaa-mysql", [])):
            with self.assertRaises(AssertionError):
                fixture_target(SimpleNamespace(mysql=name), created)

    def test_corrupt_backup_is_rejected_before_any_database_write(self):
        backend = SimpleNamespace(mysql="onehub-upgrade-aaaaaaaaaaaa-mysql", sql=Mock())
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "backup.sql"
            path.write_text("synthetic changed backup", encoding="utf-8")
            with self.assertRaises(AssertionError):
                restore_database(backend, [("container", backend.mysql)], path, "0" * 64)
            backend.sql.assert_not_called()

    def test_restore_requires_exact_dump_roundtrip(self):
        backend = SimpleNamespace(mysql="onehub-upgrade-aaaaaaaaaaaa-mysql", sql=Mock())
        data = "synthetic backup"
        with tempfile.TemporaryDirectory() as tmp, patch("upgrade.database_dump", return_value=data):
            path = Path(tmp) / "backup.sql"
            path.write_text(data, encoding="utf-8")
            restore_database(backend, [("container", backend.mysql)], path, hashlib.sha256(data.encode()).hexdigest())
            backend.sql.assert_called_once_with(data)

    def test_restore_detects_schema_or_data_mismatch(self):
        backend = SimpleNamespace(mysql="onehub-upgrade-aaaaaaaaaaaa-mysql", sql=Mock())
        data = "synthetic backup"
        with tempfile.TemporaryDirectory() as tmp, patch("upgrade.database_dump", return_value="different restored state"):
            path = Path(tmp) / "backup.sql"
            path.write_text(data, encoding="utf-8")
            with self.assertRaises(AssertionError):
                restore_database(backend, [("container", backend.mysql)], path, hashlib.sha256(data.encode()).hexdigest())


if __name__ == "__main__":
    unittest.main()
