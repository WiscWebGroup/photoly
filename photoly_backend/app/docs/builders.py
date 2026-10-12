"""Small documentation builders. They never wrap or validate live requests."""
from copy import deepcopy
from .schemas import INTEGER, STRING, obj, ref, nullable


def parameter(name, schema=INTEGER, description=None, required=True, where="query", example=None):
    value = {"name": name, "in": where, "required": required, "schema": deepcopy(schema),
             "description": description or ("Identifier of a resource owned by the current user." if name.endswith("Id") else name)}
    if example is not None:
        value["example"] = example
    return value


def token_parameter(where="path", kind="login"):
    return parameter("token", STRING, {"login": "Login session token returned by /user/signIn or /user/signUp. Not a photo share token or API credential.",
                                      "api": "API credential from /cred/create and /cred/query. Not a login session or photo share token.",
                                      "share": "Per-photo token returned in Photo.token. Required for visibility 1; optional for public visibility 2."}[kind],
                     required=kind != "share", where=where)


def ids_parameter():
    return {**parameter("tagIds", {"type": "array", "items": INTEGER},
                        "Tag IDs. Repeated parameters (tagIds=1&tagIds=2) and comma-separated values (tagIds=1,2) are both accepted.",
                        required=False, example=[1, 2]), "style": "form", "explode": True}


def pages():
    return [parameter("page", {"type": "integer", "minimum": 1}, "One-based page number.", example=1),
            parameter("rowsPerPage", {"type": "integer", "minimum": 1}, "Number of records per page.", example=20)]


def json_body(schema, example):
    return {"required": True, "content": {"application/json": {"schema": schema, "example": example}}}


def upload_body(multiple=False, avatar=False):
    file = {"type": "string", "format": "binary", "description": "Choose a local file. Request limit: 600 MiB by default."}
    if avatar:
        properties = {"file": file}
    elif multiple:
        properties = {"files": {"type": "array", "items": file, "minItems": 1},
                      "photosStr": {"type": "string", "description": "JSON-encoded array of PhotoUploadMetadata objects, in the same order and with the same count as files. This part must be text, not a nested multipart object.",
                                    "example": '[{"nsId":1,"visibility":0},{"nsId":1,"visibility":0}]'}}
    else:
        properties = {"file": file, "photo": {"type": "string",
                      "description": "JSON-encoded PhotoUploadMetadata object. nsId must belong to the authenticated user. This part must be text.",
                      "example": '{"nsId":1,"visibility":0}'}}
    return {"required": True, "description": "Get an owned namespace ID from /namespace/getRoot before uploading. Visibility defaults to private (0)." if not avatar else "The file replaces the user's avatar.",
            "content": {"multipart/form-data": {"schema": obj(properties)}}}


def operation(tag, name, method, path, summary, value=INTEGER, example=1, *, description="", params=(), body=None,
              auth="header", shape="result", missing=None, failure=-1, page_missing=0, code=200, form=False,
              mime=None, download=None, ranges=False, raw_content=None, raw_empty=False, errors=None):
    parameters = deepcopy(list(params))
    details = description
    if tag == "Admin" and name != "is_admin":
        details += " Administrator role required."
    if shape in ("result", "page", "token"):
        details += " Business failures normally use HTTP 200: inspect msgCode, not just the HTTP status."
    if form and parameters:
        # Query and form are alternatives; requiring either individually would reject valid calls in the UI.
        fields = {p["name"]: p["schema"] for p in parameters if p["in"] == "query"}
        required = [p["name"] for p in parameters if p["in"] == "query" and p["required"]]
        for p in parameters:
            if p["in"] == "query":
                p["required"] = False
                p["description"] += " May instead be sent as an application/x-www-form-urlencoded field."
        form_schema = obj(fields, required)
        form_example = {p["name"]: p.get("example", 1 if p["schema"].get("type") == "integer" else "example") for p in parameters if p["in"] == "query"}
        body = {"required": False, "content": {"application/x-www-form-urlencoded": {"schema": form_schema, "example": form_example}}}
        details += " Supply the documented inputs via query parameters OR form fields. Query values take precedence."
        if required:
            details += " Required for the intended operation: " + ", ".join(required) + "."
    responses = {}
    if shape in ("result", "page", "token"):
        props = {"t": nullable(value), "msgCode": {"type": "integer", "const": code}}
        success = {"t": example, "msgCode": code}
        fail = {"t": failure, "msgCode": 400}
        unauthorized = {"t": missing, "msgCode": 403}
        failure_schema = ref("BusinessFailure")
        if shape == "page":
            props["pageNum"] = {"type": "integer", "description": "Total pages, not the requested page index."}
            success["pageNum"] = 1
            fail.update(t=None, pageNum=0)
            unauthorized["pageNum"] = page_missing
            failure_schema = obj({"t": {"type": "null"}, "pageNum": INTEGER, "msgCode": {"enum": [400, 403]}})
        if shape == "token":
            props["token"] = nullable(STRING)
            success["token"] = "example-login-session-token"
            fail.update(t="Invalid Email or Password", token=None)
            failure_schema = obj({"t": STRING, "token": {"type": "null"}, "msgCode": {"enum": [400, 401, 403]}})
        examples = {"success": {"value": success}, "businessFailure": {"value": fail}}
        if auth == "header":
            examples["unauthenticated"] = {"value": unauthorized}
        if errors:
            examples.update({key: {"value": v} for key, v in errors.items()})
        responses["200"] = {"description": "Legacy response; msgCode reports the result.", "content": {
            "application/json": {"schema": {"anyOf": [obj(props), failure_schema]}, "examples": examples}}}
    elif shape == "raw":
        content = deepcopy(raw_content or {"application/json": {"schema": value, "example": example}})
        if raw_empty:
            content["text/html"] = {"schema": {"type": "string", "const": ""}, "example": ""}
        responses["200"] = {"description": "Direct response with no t/msgCode envelope. " + ("Some failures return an empty body." if raw_empty else ""), "content": content}
    else:
        responses["200"] = {"description": "File bytes. Cache-Control: private, no-store.",
                            "content": {mime: {"schema": {"type": "string", "format": "binary"}}}}
        for status, text in (("400", "Invalid input or resource ownership."), ("403", "Missing/invalid token or private media."), ("404", "Resource or file is missing.")):
            responses[status] = {"description": text + " The response may have an empty body."}
    if download:
        responses["200"]["headers"] = {"Content-Disposition": {"schema": STRING, "description": "Attachment download.", "example": "attachment; filename=" + download}}
    if ranges:
        parameters.append(parameter("Range", STRING, "Single byte range, e.g. bytes=0-1023, bytes=1024- or bytes=-1024.", False, "header", "bytes=0-1023"))
        responses["206"] = {"description": "Requested byte range.", "content": {mime: {"schema": {"type": "string", "format": "binary"}}},
                            "headers": {"Content-Range": {"schema": STRING, "example": "bytes 0-1023/4096"}, "Accept-Ranges": {"schema": STRING, "example": "bytes"}}}
        responses["416"] = {"description": "Unsatisfiable range.", "headers": {"Content-Range": {"schema": STRING, "example": "bytes */4096"}},
                            "content": {"application/json": {"schema": ref("HttpError"), "example": {"t": None, "msgCode": 416}}}}
    responses["503"] = {"description": "Database or Redis unavailable.", "content": {"application/json": {"schema": ref("HttpError"), "example": {"t": None, "msgCode": 503}}}}
    if body:
        responses["413"] = {"description": "Request exceeds the configured size limit.", "content": {"application/json": {"schema": ref("HttpError"), "example": {"t": None, "msgCode": 413}}}}
    result = {"tags": [tag], "operationId": tag.lower().replace("-", "_") + "_" + name,
              "summary": summary, "description": details.strip() or summary, "parameters": parameters,
              "security": [{"SessionToken": []}] if auth == "header" else [], "responses": responses}
    if body:
        result["requestBody"] = body
    return path, method.lower(), result
