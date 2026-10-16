import math
import re
from datetime import datetime, timezone
from zoneinfo import ZoneInfo


def device_name(make, model):
    """A model is required: a brand alone is not a device identification."""
    if not isinstance(model, str) or not model.strip():
        return None
    model = ' '.join(model.split())
    make = ' '.join(str(make or '').split())
    while make and model.casefold().startswith((make + ' ' + make).casefold()):
        model = model[len(make) + 1:]
    if make and not model.casefold().startswith(make.casefold()):
        model = make + ' ' + model
    # iPhone/iPad model names already identify the manufacturer unambiguously.
    model = re.sub(r'^Apple (?=iPhone|iPad)', '', model, flags=re.I)
    return model[:255]


def coordinates(data):
    try:
        lat, lon = float(data['GPSLatitude']), float(data['GPSLongitude'])
        if math.isfinite(lat) and math.isfinite(lon) and -90 <= lat <= 90 and -180 <= lon <= 180:
            return lat, lon
    except (KeyError, ValueError, TypeError):
        pass
    return None, None


def capture_time(data, city_zone=None, app_zone='America/New_York'):
    for key, offset_key in (('SubSecDateTimeOriginal', 'OffsetTimeOriginal'), ('DateTimeOriginal', 'OffsetTimeOriginal'),
                            ('CreationDate', 'OffsetTimeDigitized'), ('CreateDate', 'OffsetTimeDigitized'),
                            ('MediaCreateDate', 'OffsetTimeDigitized')):
        raw = data.get(key)
        if not raw:
            continue
        value = re.sub(r'^(\d{4}):(\d{2}):(\d{2})', r'\1-\2-\3', str(raw).strip())
        try:
            taken = datetime.fromisoformat(value.replace('Z', '+00:00'))
            if taken.year < 1900:
                continue
            source = 'explicit'
            if taken.tzinfo is None and re.fullmatch(r'[+-]\d{2}:\d{2}', str(data.get(offset_key, ''))):
                taken = datetime.fromisoformat(value + data[offset_key])
            if taken.tzinfo is None:
                zone = city_zone or app_zone
                taken = taken.replace(tzinfo=ZoneInfo(zone))
                source = 'city' if city_zone else 'application'
                # Reject nonexistent wall times rather than inventing a capture time.
                if taken.astimezone(timezone.utc).astimezone(taken.tzinfo).replace(tzinfo=None) != taken.replace(tzinfo=None):
                    continue
            return taken.astimezone(timezone.utc), {'timeField': key, 'timezoneSource': source,
                'timezone': str(taken.tzinfo), 'localDate': taken.date().isoformat()}
        except (ValueError, TypeError):
            continue
    return None, {}


def extract(data, gazetteer, app_zone):
    lat, lon = coordinates(data)
    city = gazetteer.nearest(lat, lon) if lat is not None else None
    taken, evidence = capture_time(data, city['timezone'] if city else None, app_zone)
    return {'captured_at': taken, 'latitude': lat, 'longitude': lon, 'facts': {
        **evidence, 'camera': device_name(data.get('Make'), data.get('Model')),
        'lens': device_name(data.get('LensMake'), data.get('LensModel')), 'city': city}}
