"""Public wire schemas and fictitious examples, independent of ORM and test fixtures."""
from copy import deepcopy


def ref(name):
    return {"$ref": "#/components/schemas/" + name}


def array(items):
    return {"type": "array", "items": items}


def nullable(schema):
    return {"anyOf": [schema, {"type": "null"}]}


def obj(properties, required=None):
    return {"type": "object", "properties": properties,
            "required": list(properties) if required is None else required}


INTEGER = {"type": "integer"}
STRING = {"type": "string"}
BOOLEAN = {"type": "boolean"}
NUMBER = {"type": "number"}
LOCAL_TIME = {"type": "string", "pattern": r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?$",
              "description": "Local date and time in the configured TIMEZONE, without a UTC offset.",
              "examples": ["2026-01-15T12:30:00"]}
DATE = {"type": "string", "format": "date"}
VISIBILITY = {"type": "integer", "enum": [0, 1, 2],
              "description": "0: private; 1: photo share token required; 2: public."}
NAME = {"type": "string", "minLength": 1, "maxLength": 255,
        "description": "Nonblank name. No slash, backslash, NUL, CR, LF, '.' or '..'."}
PASSWORD = {"type": "string", "minLength": 1, "format": "password", "writeOnly": True}
EMAIL = {"type": "string", "description": "Email address, e.g. reader@example.com."}

ENTITY_PROPERTIES = {
    "User": {"userId": INTEGER, "userName": STRING, "email": STRING, "createDate": LOCAL_TIME,
             "role": STRING, "password": {"type": "null", "description": "Always null; neither password nor hash is exposed."}, "uuid": STRING},
    "Namespace": {"nsId": INTEGER, "nsName": STRING, "nsParentId": INTEGER, "userId": INTEGER},
    "Photo": {"photoId": INTEGER, "photoName": STRING, "uploadDate": LOCAL_TIME, "format": STRING,
              "nsId": INTEGER, "visibility": VISIBILITY, "token": STRING, "photoUuid": STRING, "userId": INTEGER},
    "Gallery": {"gaId": INTEGER, "gaName": STRING, "userId": INTEGER, "createDate": LOCAL_TIME,
                "coverId": INTEGER, "coverColor": STRING},
    "Tag": {"tagId": INTEGER, "userId": INTEGER, "tagName": STRING},
    "Cred": {"credId": INTEGER, "userId": INTEGER, "token": STRING, "authorization": STRING},
    "PhotoExtra": {"extraPhotoId": INTEGER, "fPhotoId": INTEGER, "extraPhotoGpsLat": NUMBER,
                   "extraPhotoGpsLon": NUMBER, "extraPhotoCamera": STRING, "extraPhotoLens": STRING,
                   "extraPhotoAperture": NUMBER, "extraPhotoFocal": INTEGER, "extraPhotoIso": INTEGER,
                   "extraPhotoShutterSec": INTEGER, "extraPhotoTakenDate": LOCAL_TIME, "extraPhotoNote": STRING},
    "GalleryPhoto": {"gpId": INTEGER, "gaId": INTEGER, "photoId": INTEGER},
    "TagPhoto": {"tpId": INTEGER, "userId": INTEGER, "tagId": INTEGER, "photoId": INTEGER},
    "NamespacePhoto": {"npId": INTEGER, "nsId": INTEGER, "photoId": INTEGER},
    "Setting": {"settingId": INTEGER, "settingName": STRING, "settingValue": STRING, "note": STRING},
}
SCHEMAS = {name: obj({key: (deepcopy(value) if key == next(iter(properties)) or key == "password" else nullable(deepcopy(value)))
                     for key, value in properties.items()}) for name, properties in ENTITY_PROPERTIES.items()}
SCHEMAS.update({
    "BusinessFailure": obj({"t": {"type": ["integer", "string", "boolean", "null"]},
                            "msgCode": {"type": "integer", "enum": [400, 401, 402, 403, 404]}}),
    "HttpError": obj({"t": {"type": ["integer", "null"]}, "msgCode": INTEGER}),
    "PhotoUploadMetadata": obj({"nsId": INTEGER, "visibility": VISIBILITY}, ["nsId"]),
    "PhotoUpdate": obj({"photoId": INTEGER, "photoName": NAME, "visibility": VISIBILITY}, ["photoId"]),
    "SignIn": obj({"email": EMAIL, "password": PASSWORD}),
    "SignUp": obj({"email": EMAIL, "password": PASSWORD, "userName": STRING}, ["email", "password"]),
    "NamespaceCreate": obj({"nsName": NAME, "nsParentId": INTEGER}),
    "NamespaceRename": obj({"nsId": INTEGER, "nsName": NAME}),
    "NamespaceMove": obj({"nsId": INTEGER, "nsParentId": INTEGER}),
    "GalleryCreate": obj({"gaName": NAME, "coverId": nullable(INTEGER), "coverColor": nullable(STRING)}, ["gaName"]),
    "GalleryUpdate": obj({"gaId": INTEGER, "gaName": NAME, "coverId": nullable(INTEGER), "coverColor": nullable(STRING)}, ["gaId", "gaName"]),
    "TagCreate": obj({"tagName": NAME}),
    "TagUpdate": obj({"tagId": INTEGER, "tagName": NAME}),
    "CredCreate": obj({"authorization": {"type": "string", "description": "Any nonempty combination of C (create), R (read), D (delete); stored uppercase in CRD order."}}),
    "CredUpdate": obj({"credId": INTEGER, "authorization": STRING}),
    "AdminUserCreate": obj({"userName": STRING, "email": EMAIL, "password": PASSWORD,
                            "role": {"type": "string", "enum": ["user", "admin"], "default": "user"}}, ["userName", "email", "password"]),
    "FolderTraceEntry": obj({"nsId": INTEGER, "nsName": STRING}),
    "PhotoTag": obj({"photo_id": INTEGER, "tag_id": INTEGER, "tag_name": STRING}),
    "PhotoGallery": obj({"photo_id": INTEGER, "ga_id": INTEGER, "ga_name": STRING}),
    "MapPhoto": obj({"photo_id": INTEGER, "photo_name": nullable(STRING), "upload_date": nullable(DATE),
                     "format": nullable(STRING), "ns_id": nullable(INTEGER), "token": nullable(STRING),
                     "photo_uuid": nullable(STRING), "extra_photo_gps_lat": nullable(NUMBER),
                     "extra_photo_gps_lon": nullable(NUMBER), "extra_photo_taken_date": nullable(DATE)}),
    "Exif": {"type": "object", "additionalProperties": STRING,
             "description": "Available metadata only; keys include Image_Width, Image_Height, File_Size, Make, Model, Lens_Make, Lens_Model, GPS_Latitude, GPS_Longitude, Aperture_Value, Shutter_Speed_Value, ISO_Speed_Ratings and Time. All values are strings."},
})
EXAMPLES = {
    "User": {"userId": 1, "userName": "Reader", "email": "reader@example.com", "createDate": "2026-01-15T12:30:00",
             "role": "user", "password": None, "uuid": "00000000-0000-4000-8000-000000000001"},
    "Namespace": {"nsId": 1, "nsName": "/", "nsParentId": -1, "userId": 1},
    "Photo": {"photoId": 1, "photoName": "landscape", "uploadDate": "2026-01-15T12:30:00", "format": "jpg",
              "nsId": 1, "visibility": 1, "token": "example-photo-share-token", "photoUuid": "00000000-0000-4000-8000-000000000002", "userId": 1},
    "Gallery": {"gaId": 1, "gaName": "My Favorite", "userId": 1, "createDate": "2026-01-15T12:30:00", "coverId": 2, "coverColor": "#FFFFFF"},
    "Tag": {"tagId": 1, "userId": 1, "tagName": "landscape"},
    "Cred": {"credId": 1, "userId": 1, "token": "example-api-credential", "authorization": "CRD"},
    "MapPhoto": {"photo_id": 1, "photo_name": "landscape", "upload_date": "2026-01-15", "format": "jpg", "ns_id": 1,
                 "token": "example-photo-share-token", "photo_uuid": "00000000-0000-4000-8000-000000000002",
                 "extra_photo_gps_lat": None, "extra_photo_gps_lon": None, "extra_photo_taken_date": None},
}
