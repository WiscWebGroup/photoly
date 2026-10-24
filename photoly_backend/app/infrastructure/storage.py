import hashlib
import logging
import os
import shutil
import tempfile
from pathlib import Path
from uuid import uuid4
from zipfile import ZipFile, ZIP_DEFLATED
from PIL import Image, ImageOps, UnidentifiedImageError
from pillow_heif import register_heif_opener
register_heif_opener(thumbnails=False)
from app.services.base import BusinessError

log = logging.getLogger(__name__)


class FileMutation:
    """Compensate filesystem changes if the surrounding database transaction fails."""
    def __init__(self, root):
        self.root = root
        self.created = []
        self.removed = []
        self.work = None
        self.original_sizes = {}

    def __enter__(self):
        staging = self.root / ".staging"
        staging.mkdir(parents=True, exist_ok=True)
        self.work = Path(tempfile.mkdtemp(dir=staging))
        return self

    def install(self, stream, destination):
        temporary = self.work / str(uuid4())
        digest = hashlib.sha256()
        with temporary.open("wb") as output:
            while chunk := stream.read(1024 * 1024):
                digest.update(chunk)
                output.write(chunk)
        destination.parent.mkdir(parents=True, exist_ok=True)
        self.remove(destination)
        os.replace(temporary, destination)
        self.created.append(destination)
        return digest.hexdigest()

    def remove(self, path):
        self.original_sizes.setdefault(path, path.stat().st_size if path.is_file() else 0)
        if path.exists():
            backup = self.work / str(uuid4())
            os.replace(path, backup)
            self.removed.append((path, backup))
            return 1
        return 0

    def delta(self, directory):
        return sum((path.stat().st_size if path.is_file() else 0) - before
                   for path, before in self.original_sizes.items() if path.parent == directory)

    def __exit__(self, kind, value, traceback):
        if kind is not None:
            for path in reversed(self.created):
                path.unlink(missing_ok=True)
            for original, backup in reversed(self.removed):
                original.parent.mkdir(parents=True, exist_ok=True)
                os.replace(backup, original)
        try:
            shutil.rmtree(self.work)
        except OSError:
            # After DB commit, cleanup failure must not report a rolled-back operation.
            log.exception("Could not clean transaction staging directory %s", self.work)


class LocalStorage:
    def __init__(self, root):
        self.root = Path(root).resolve()

    def mutation(self):
        return FileMutation(self.root)

    def usage(self, user_uuid):
        """Bytes in one user's media folder, and the size of the disk holding all media."""
        used = 0
        folder = self.root / user_uuid
        for directory, _, names in os.walk(folder):
            for name in names:
                try:
                    used += os.lstat(os.path.join(directory, name)).st_size
                except OSError:
                    pass  # Removed while counting.
        self.root.mkdir(parents=True, exist_ok=True)
        return used, shutil.disk_usage(self.root).total

    def path(self, user_uuid, filename):
        # Check lexical components as well as symlink resolution.
        for part in (user_uuid, filename):
            if not isinstance(part, str) or part in ("", ".", "..") or any(c in part for c in ("/", "\\", "\x00")):
                raise BusinessError(-1)
        path = self.root / user_uuid / filename
        if not path.resolve().is_relative_to(self.root):
            raise BusinessError(-1)
        return path

    def media(self, user_uuid, photo, thumbnail=False):
        name = photo.photo_uuid + ("_thumbnail" if thumbnail else "") + "." + photo.format
        return self.path(user_uuid, name)

    def thumbnail(self, original, destination, files):
        if original.suffix.lower() not in {".jpg", ".jpeg", ".png", ".gif", ".jfif", ".ico", ".bmp", ".tif", ".tiff", ".webp", ".heic", ".heif"}:
            return
        try:
            with Image.open(original) as image:
                output = ImageOps.exif_transpose(image).convert("RGB")
                output.thumbnail((400, 400), Image.Resampling.LANCZOS)
                fmt = "JPEG" if image.format in {"HEIF", "MPO"} else image.format
                temporary = files.work / str(uuid4())
                output.save(temporary, format=fmt)
                with temporary.open("rb") as stream:
                    files.install(stream, destination)
        except (UnidentifiedImageError, OSError, ValueError, Image.DecompressionBombError) as error:
            raise BusinessError(-1) from error

    def archive(self, entries):
        """Build each archive independently; original files are never copied into named folders."""
        archive = tempfile.SpooledTemporaryFile(max_size=8 * 1024 * 1024, mode="w+b")
        names = set()
        try:
            with ZipFile(archive, "w", ZIP_DEFLATED) as output:
                for path, name, identifier in entries:
                    if not path.is_file():
                        raise BusinessError(None, 404)
                    candidate = name
                    index = 0
                    while candidate in names:
                        index += 1
                        part = Path(name)
                        candidate = str(part.with_name(f"{part.stem} ({identifier}-{index}){part.suffix}"))
                    names.add(candidate)
                    try:
                        output.write(path, candidate)
                    except FileNotFoundError as error:
                        # Another request may delete a file after the existence check.
                        raise BusinessError(None, 404) from error
            archive.seek(0)
            return archive
        except Exception:
            archive.close()
            raise
