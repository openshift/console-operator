"""Exercise the production archive functions without starting the HTTP server."""
import ast
import os
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest import mock
import zipfile


tree = ast.parse(sys.stdin.read())
functions = ast.Module(
    body=[
        node for node in tree.body
        if isinstance(node, ast.FunctionDef)
        and node.name in ("create_archives", "create_all_archives")
    ],
    type_ignores=[],
)
namespace = {"os": os, "tarfile": tarfile, "zipfile": zipfile}
exec(compile(functions, "<downloads-server>", "exec"), namespace)


class ArchivesTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.source = Path(self.temp.name) / "oc"
        self.contents = b"client binary\n" * 1024
        self.source.write_bytes(self.contents)
        self.root = str(Path(self.temp.name) / "download")
        self.archive = ("amd64", "linux", str(self.source), "oc", self.root)

    def check_archives(self):
        with tarfile.open(self.root + ".tar") as archive:
            self.assertEqual(archive.extractfile("oc").read(), self.contents)
        with zipfile.ZipFile(self.root + ".zip") as archive:
            self.assertEqual(archive.read("oc"), self.contents)

    def test_source_advised_after_both_archives_are_complete(self):
        descriptors = []

        def advise(fd, offset, length, advice):
            # Verify the descriptor targets the source, not either output.
            self.assertEqual(os.fstat(fd).st_ino, self.source.stat().st_ino)
            self.assertEqual((offset, length, advice), (0, 0, os.POSIX_FADV_DONTNEED))
            self.check_archives()
            descriptors.append(fd)

        with mock.patch.object(os, "posix_fadvise", side_effect=advise) as advised:
            namespace["create_archives"](*self.archive)
        advised.assert_called_once()
        with self.assertRaises(OSError):
            os.fstat(descriptors[0])

    def test_advice_failure_does_not_fail_archive_creation(self):
        with mock.patch.object(os, "posix_fadvise", side_effect=OSError("unsupported")):
            namespace["create_archives"](*self.archive)
        self.check_archives()

    def test_archive_failure_still_advises_source(self):
        with mock.patch.object(tarfile, "open", side_effect=OSError("write failed")):
            with mock.patch.object(os, "posix_fadvise") as advised:
                namespace["create_archives"](*self.archive)
        advised.assert_called_once()

    def test_worker_continues_after_failure(self):
        missing = ("amd64", "linux", str(self.source) + ".missing", "oc", self.root)
        with mock.patch.object(os, "posix_fadvise"):
            namespace["create_all_archives"]([missing, self.archive])
        self.check_archives()

    def test_worker_processes_each_artifact_in_order(self):
        archives = [self.archive, ("arm64", "linux", "other", "oc", "other")]
        with mock.patch.dict(namespace, create_archives=mock.Mock()) as env:
            namespace["create_all_archives"](archives)
            self.assertEqual(
                env["create_archives"].call_args_list,
                [mock.call(*archive) for archive in archives],
            )


if __name__ == "__main__":
    unittest.main()
