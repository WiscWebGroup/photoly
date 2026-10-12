"""ExifTool numeric data normalized into the existing metadata-extractor API keys."""
import json
import logging
import subprocess
from datetime import datetime
from fractions import Fraction
from app.entities import PhotoExtra

log = logging.getLogger(__name__)


class MetadataReader:
    def __init__(self, executable="exiftool"):
        self.executable = executable

    def read(self, path):
        try:
            process = subprocess.run([self.executable, "-json", "-n", str(path.resolve())],
                                     capture_output=True, text=True, timeout=30, check=False)
            records = json.loads(process.stdout or "[]")
            return records[0] if records else {}
        except (OSError, subprocess.TimeoutExpired, ValueError, IndexError):
            log.warning("Metadata unavailable for %s", path.name)
            return {}

    def public(self, data, path):
        fields = {"Make": "Make", "Model": "Model", "LensMake": "Lens_Make", "LensModel": "Lens_Model",
                  "Artist": "Artist", "ColorSpace": "Color_Space", "GPSAltitude": "GPS_Altitude",
                  "GPSAltitudeRef": "GPS_Altitude_Ref", "GPSVersionID": "GPS_Version_ID",
                  "GPSLongitudeRef": "GPS_Longitude_Ref", "GPSLatitudeRef": "GPS_Latitude_Ref",
                  "XResolution": "X_Resolution", "YResolution": "Y_Resolution", "BitsPerSample": "Data_Precision"}
        result = {target: str(data[key]) for key, target in fields.items() if key in data}
        for source, target in (("GPSLatitude", "GPS_Latitude"), ("GPSLongitude", "GPS_Longitude")):
            if source in data:
                result[target] = f"{float(data[source]):.4f}"
        for key, unit in (("ImageWidth", "pixels"), ("ImageHeight", "pixels"), ("FocalLength", "mm")):
            if key in data:
                target = {"ImageWidth": "Image_Width", "ImageHeight": "Image_Height", "FocalLength": "Focal_Length"}[key]
                result[target] = f"{data[key]} {unit}"
        aperture = data.get("ApertureValue", data.get("FNumber"))
        if aperture is not None:
            result["Aperture_Value"] = f"f/{aperture:g}"
        shutter = data.get("ExposureTime", data.get("ShutterSpeedValue"))
        if shutter:
            result["Shutter_Speed_Value"] = f"{Fraction(float(shutter)).limit_denominator(100000)} sec"
        if "ISO" in data:
            result["ISO_Speed_Ratings"] = str(data["ISO"])
        stamp = data.get("ModifyDate", data.get("DateTimeOriginal", data.get("CreateDate")))
        if stamp:
            result["Time"] = str(stamp)
        result["File_Size"] = f"{path.stat().st_size / 1024 / 1024:.2f} MB"
        return result

    def extra(self, photo_id, data):
        stamp = data.get("ModifyDate", data.get("DateTimeOriginal", data.get("CreateDate")))
        taken = None
        if stamp:
            try:
                taken = datetime.strptime(str(stamp)[:19], "%Y:%m:%d %H:%M:%S")
            except ValueError:
                pass
        camera = " ".join(str(data[key]) for key in ("Make", "Model") if data.get(key)) or None
        lens = " ".join(str(data[key]) for key in ("LensMake", "LensModel") if data.get(key)) or None
        def number(key, integer=False):
            try:
                return int(float(data[key])) if integer else float(data[key])
            except (ValueError, TypeError, KeyError):
                return None
        shutter = number("ExposureTime") or number("ShutterSpeedValue")
        return PhotoExtra(f_photo_id=photo_id,
            extra_photo_gps_lat=number("GPSLatitude"), extra_photo_gps_lon=number("GPSLongitude"),
            extra_photo_camera=camera, extra_photo_lens=lens,
            extra_photo_aperture=number("ApertureValue") or number("FNumber"),
            extra_photo_focal=number("FocalLength", True), extra_photo_iso=number("ISO", True),
            extra_photo_shutter_sec=round(1 / shutter) if shutter and shutter > 0 else None,
            extra_photo_taken_date=taken)
