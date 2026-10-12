import logging
import os
import shutil
import tempfile
from pathlib import Path
from uuid import uuid4
from zipfile import ZipFile, ZIP_DEFLATED
from PIL import Image, UnidentifiedImageError
from app.services.base import BusinessError

log = logging.getLogger(__name__)


class FileMutation:
    """Compensate filesystem changes if the surrounding database transaction fails."""
    def __init__(self, root):
        self.root = root
        self.created = []
        self.removed = []
        self.work = None

    def __enter__(self):
        staging = self.root / ".staging"
        staging.mkdir(parents=True, exist_ok=True)
        self.work = Path(tempfile.mkdtemp(dir=staging))
        return self

    def install(self, stream, destination):
        temporary = self.work / str(uuid4())
        with temporary.open("wb") as output:
            shutil.copyfileobj(stream, output, 1024 * 1024)
        destination.parent.mkdir(parents=True, exist_ok=True)
        self.remove(destination)
        os.replace(temporary, destination)
        self.created.append(destination)

    def remove(self, path):
        if path.exists():
            backup = self.work / str(uuid4())
            os.replace(path, backup)
            self.removed.append((path, backup))
            return 1
        return 0

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
        if original.suffix.lower() not in {".jpg", ".jpeg", ".png", ".gif", ".jfif", ".ico", ".bmp", ".tif", ".tiff", ".webp"}:
            return
        try:
            with Image.open(original) as image:
                width, height = image.size
                if width > 400 or height > 250:
                    size = (400, max(1, 400 * height // width))
                else:
                    size = image.size
                output = image.convert("RGB").resize(size, Image.Resampling.LANCZOS)
                fmt = image.format
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
