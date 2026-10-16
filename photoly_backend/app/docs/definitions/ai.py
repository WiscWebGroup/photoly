"""AI endpoints use real HTTP errors, unlike legacy business operations."""
from ..schemas import obj, ref, array, STRING, INTEGER, BOOLEAN, NUMBER, nullable
from ..builders import parameter, json_body
from app.ai.limits import LIMIT_DEFAULTS, LIMIT_BOUNDS

LIMIT_FIELDS = {key: {"type":"integer", "minimum":bounds[0], "maximum":bounds[1], "default":LIMIT_DEFAULTS[key]}
                for key,bounds in LIMIT_BOUNDS.items()}
LIMIT_FIELDS["background_interval_seconds"]["description"] = "Minimum seconds between background job claims. Zero disables the extra delay; the control loop still polls every 2 seconds. Applies without reloading the model."
LIMIT_FIELDS["background_max_per_minute"]["description"] = "Background attempts per rolling 60-second window, including retries. Zero disables this cap. Persisted across worker restarts."
LIMIT_FIELDS["cpu_threads"]["description"] = "PyTorch CPU compute threads, also used for CPU work in CUDA and MPS modes. Unload the model before changing; next load applies it. Not a container CPU quota."
for key in LIMIT_FIELDS:
    if key.endswith("_percent"):
        LIMIT_FIELDS[key]["description"] = "Background admission threshold, not a utilization cap. Resume must be strictly less than pause."
    elif key.endswith("_mib"):
        LIMIT_FIELDS[key]["description"] = "Minimum available MiB for background admission. Resume must be strictly greater than pause."
BACKGROUND_LIMIT_STATUS=obj({"lastStartedAt":nullable(STRING),"startsLastMinute":INTEGER,
    "nextAllowedAt":nullable(STRING),"reason":nullable(STRING),"maxConcurrency":{"type":"integer","const":1},"batchSize":{"type":"integer","const":1}})

SIMILARITY_THRESHOLD={"type":["number","null"],"minimum":-1,"maximum":1,"default":None,
    "description":"Minimum cosine similarity for image-query and similar-media search, inclusive. Null disables filtering (default). Text search is unaffected. Changes invalidate existing recommendation cursors with search_settings_changed (409). No reindex or model reload required."}

COVERAGE=obj({"eligible":INTEGER,"supported":INTEGER,"ready":INTEGER,"pending":INTEGER,"failed":INTEGER,"unsupported":INTEGER,"ratio":NUMBER})
SETTINGS=obj({"generation":INTEGER,"similarity_threshold":SIMILARITY_THRESHOLD,"enabled":BOOLEAN,"new_user_default":BOOLEAN,"smart_tag_enabled":BOOLEAN,"maintenance":BOOLEAN,
    "search_profile_id":nullable(INTEGER),"building_profile_id":nullable(INTEGER),"quota_bytes":INTEGER,
    "scheduler":STRING,"runtime":STRING,"loaded_profile_id":nullable(INTEGER),"device":STRING,"precision":STRING,**LIMIT_FIELDS})
RESOURCE=obj({"sampled_at":STRING,"cpu_percent":nullable(NUMBER),"host_cpu_percent":nullable(NUMBER),"container_cpu_percent":nullable(NUMBER),"cpu_capacity":nullable(NUMBER),"memory_available":INTEGER,"memory_total":INTEGER,"disk_free":INTEGER,"gpu_percent":nullable(NUMBER),"vram_available":nullable(INTEGER),"vram_total":nullable(INTEGER),"missing":array(STRING),"free_score":NUMBER}, [])
DEVICE=obj({"device":STRING,"precision":STRING,"available":nullable(BOOLEAN),"reason":nullable(STRING)})
FIELD=obj({"default":nullable(NUMBER),"minimum":NUMBER,"maximum":NUMBER,"unit":STRING,"apply":STRING,"pairedWith":nullable(STRING),"relation":nullable(STRING)})
ADMIN_CAPS=obj({"fields":{"type":"object","additionalProperties":FIELD},"devices":array(DEVICE),"verified":array(obj({"fingerprint":STRING,"device":STRING,"precision":STRING,"cpu_threads":INTEGER})),"maxConcurrency":INTEGER,"batchSize":INTEGER})
QUOTA=obj({"estimated_bytes":INTEGER,"reserved_bytes":INTEGER,"physical_bytes":INTEGER})
OBSERVED=obj({"state":STRING,"reason":{"type":["string","array","null"],"items":STRING},"fingerprint":nullable(STRING),"generation":INTEGER,"config":obj({"device":STRING,"precision":STRING,"cpu_threads":INTEGER}),"resources":RESOURCE,"devices":array(DEVICE),"queued":INTEGER,"verified":array({"type":"object"})}, ["state"])
SERVICE=obj({"online":BOOLEAN,"heartbeatAt":nullable(STRING),"observed":OBSERVED})
SERVICES=obj({"scheduler":SERVICE,"runtime":SERVICE,"maintenance":SERVICE})
SPEC=obj({"provider":STRING,"model_id":STRING,"revision":STRING,"dimension":INTEGER,"query_instruction":STRING,"document_instruction":STRING,"max_tokens":INTEGER,"max_pixels":INTEGER,"preprocessing":STRING,"metric":STRING,"normalization":STRING,"fingerprint":STRING})
PROFILE=obj({"profileId":INTEGER,"provider":STRING,"dimension":INTEGER,"fingerprint":STRING,"spec":SPEC,"indexStatus":STRING,"createdAt":STRING})
PROFILE_STATUS=obj({**PROFILE["properties"],"coverage":COVERAGE,"activationBlockers":array(STRING),"purgeBlockers":array(STRING)})
JOB=obj({"command":nullable(STRING),"generation":nullable(INTEGER),"stage":STRING,"operationId":INTEGER,"kind":STRING,"photoId":nullable(INTEGER),"profileId":nullable(INTEGER),"status":STRING,
    "attempts":INTEGER,"result":{"type":"object"},"createdAt":STRING,"finishedAt":nullable(STRING)})
FILTERS={"topK":{"type":"integer","minimum":1,"maximum":100,"default":20},"namespaceId":INTEGER,"galleryId":INTEGER}
HITS=obj({"items":array(obj({"photo":ref("Photo"),"score":NUMBER,"matchedTimestampMs":{**INTEGER,"description":"Sampling time of the best matching video frame, in milliseconds. Omitted for images."}},["photo","score"])),"profileId":INTEGER,"coverage":COVERAGE,"requestId":STRING,"similarityThreshold":SIMILARITY_THRESHOLD})
COMMANDS=["model.load","model.unload","model.restart","scheduler.start","scheduler.pause","scheduler.resume","scheduler.stop",
    "maintenance.enter","maintenance.exit","backfill","retry_failed","rebuild","activate","purge"]


def api(path,method,name,response,*,body=None,params=(),accepted=False):
    code=202 if accepted else 200
    operation={"tags":["AI Admin" if path.startswith("/admin") else "AI Search"],"operationId":name,
        "summary":name.replace("_"," ").capitalize(),"security":[{"SessionToken":[]}],"parameters":list(params),
        "description":"Administrator role required. " if path.startswith("/admin") else "Only the authenticated user's photos and videos are searched. Video scores are the maximum cosine similarity across sampled frames; results are deduplicated by media ID.  ",
        "responses":{str(code):{"description":"Accepted; poll operation status." if accepted else "Success; partial indexing is reported in coverage.",
            "content":{"application/json":{"schema":obj({"t":response,"msgCode":{"const":code,"type":"integer"}})}}}}}
    for status in (400,401,403,404,409,413,415,429,502,503,504):
        operation["responses"][str(status)]={"description":"AI error; error contains a stable reason identifier.","content":{"application/json":{
            "schema":obj({"t":{"type":"null"},"msgCode":{"type":"integer","const":status},"error":STRING})}}}
    if body:operation["requestBody"]=body
    return path,method,operation


OPERATIONS=[
 api("/ai/capabilities","get","ai_capabilities",obj({"enabled":BOOLEAN,"globalEnabled":BOOLEAN,"ready":BOOLEAN,"reason":nullable(STRING),
     "similarityThreshold":SIMILARITY_THRESHOLD,"profileId":nullable(INTEGER),"profile":nullable(obj({"provider":STRING,"modelId":STRING,"dimension":INTEGER})),"coverage":COVERAGE,"modalities":array(STRING),"limits":obj({"textCharacters":INTEGER,"imageBytes":INTEGER,"imagePixels":INTEGER,"topK":INTEGER,"videoBytes":INTEGER,"videoFrames":INTEGER,"videoExtractionSeconds":INTEGER}),"formats":array(STRING),"videoFormats":array(STRING),"videoSamplingVersion":INTEGER})),
 api("/ai/search/text","post","ai_text_search",HITS,body=json_body(obj({"text":{"type":"string","minLength":1,"maxLength":512},**FILTERS},["text"]),{"text":"夕阳下的海滩","topK":20})),
 api("/ai/search/image","post","ai_image_search",HITS,body={"required":True,"content":{"multipart/form-data":{"schema":obj({"file":{"type":"string","format":"binary"},**FILTERS},["file"])}}}),
 api("/ai/photos/{photoId}/similar","post","ai_similar",HITS,body=json_body(obj(FILTERS,[]),{"topK":20}),params=[parameter("photoId",where="path")]),
 api("/ai/photos/{photoId}/similar/page","post","ai_similar_page",obj({**HITS["properties"],"nextCursor":nullable(STRING),"hasMore":BOOLEAN}),body=json_body({"oneOf":[{**obj({"pageSize":{"type":"integer","minimum":1,"maximum":60,"default":24}},[]),"additionalProperties":False},{**obj({"cursor":STRING}),"additionalProperties":False}]}, {"pageSize":24}),params=[parameter("photoId",where="path")]),
 api("/admin/ai/status","get","ai_admin_status",obj({"settings":SETTINGS,"services":SERVICES,"quota":QUOTA,"jobs":{"type":"object"},"coverage":COVERAGE,"buildingCoverage":nullable(COVERAGE),"backgroundLimits":BACKGROUND_LIMIT_STATUS,"smartTag":ref("SmartStatus")})),
 api("/admin/ai/capabilities","get","ai_admin_capabilities",ADMIN_CAPS),
 api("/admin/ai/settings","get","ai_get_settings",SETTINGS),
 api("/admin/ai/settings","patch","ai_patch_settings",SETTINGS,body=json_body(obj({"generation":INTEGER,"settings":obj({"similarity_threshold":SIMILARITY_THRESHOLD,"enabled":BOOLEAN,"new_user_default":BOOLEAN,"smart_tag_enabled":BOOLEAN,"quota_bytes":INTEGER,"device":{"enum":["cpu","cuda","mps"],"description":"Supported pairs: cpu/float32, cuda/bfloat16, mps/float32. Unload before changing device or precision. MPS requires the native macOS runtime and never falls back to CPU."},"precision":{"enum":["float32","bfloat16"]},**LIMIT_FIELDS},[])}),{"generation":0,"settings":{"background_interval_seconds":10,"background_max_per_minute":6}})),
 api("/admin/ai/profiles","get","ai_profiles",obj({"registry":array(SPEC),"profiles":array(PROFILE_STATUS)})),
 api("/admin/ai/profiles","post","ai_create_profile",PROFILE,body=json_body(obj({"provider":{"enum":["siglip2","qwen3_vl"]}}),{"provider":"siglip2"})),
 api("/admin/users/{userId}/ai","patch","ai_set_user",obj({"userId":INTEGER,"enabled":BOOLEAN}),params=[parameter("userId",where="path")],body=json_body(obj({"enabled":BOOLEAN}),{"enabled":True})),
 api("/admin/ai/users","get","ai_user_states",array(obj({"userId":INTEGER,"enabled":BOOLEAN})),params=[parameter("ids",STRING,"Comma-separated user IDs, at most 100.",example="1,2")]),
 api("/admin/ai/operations","post","ai_operation",JOB,accepted=True,params=[parameter("Idempotency-Key",STRING,"Unique retry key, 1–80 characters.",where="header",example="load-siglip-1")],body=json_body(obj({"command":{"enum":COMMANDS},"profileId":INTEGER,"generation":INTEGER},["command","generation"]),{"command":"model.load","profileId":1,"generation":1})),
 api("/admin/ai/operations/{operationId}","get","ai_get_operation",JOB,params=[parameter("operationId",where="path")]),
 api("/admin/ai/jobs","get","ai_jobs",obj({"items":array(JOB),"nextAfter":nullable(INTEGER),"nextBefore":nullable(INTEGER)}),params=[parameter("order",STRING,"asc (default) or desc; desc uses before.",required=False),parameter("before",INTEGER,"Exclusive descending job ID cursor.",required=False),parameter("limit",INTEGER,"1–100, default 50.",required=False),parameter("after",INTEGER,"Exclusive job ID cursor.",required=False),parameter("status",STRING,required=False),parameter("kind",STRING,required=False)]),
]


OPERATIONS += [
 api('/ai/smart/settings', 'get', 'smart_get_settings', ref('SmartSettings')),
 api('/ai/smart/settings', 'patch', 'smart_set_settings', ref('SmartSettings'), body=json_body(obj({'enabled': BOOLEAN}), {'enabled': True})),
 api('/ai/smart/status', 'get', 'smart_status', ref('SmartStatus')),
 api('/ai/smart/refresh', 'post', 'smart_refresh', obj({'queued': BOOLEAN})),
 api('/ai/smart/dismissed', 'get', 'smart_dismissed', obj({'items': array(ref('SmartSource')), 'nextAfter': nullable(INTEGER)}), params=[parameter('after', INTEGER, 'Exclusive smart item ID; up to 50 results.', required=False)]),
 api('/ai/smart/items/{itemId}/restore', 'post', 'smart_restore', obj({'restored': BOOLEAN}), params=[parameter('itemId', where='path')]),
]
for path, _, operation in OPERATIONS:
    if path.startswith('/ai/smart/'):
        operation['tags'] = ['Smart Organization']
        operation['responses']['200']['description'] = 'Success for the authenticated owner.'
        operation['description'] = 'Authenticated owner only. Automatic Smart Tag processing requires global AI, administrator Smart Tag permission, individual AI access, and user opt-in. Settings, status, dismissed items and restoration remain accessible when processing is disabled. Both Smart Tag switches default off. Disabling preserves editable results. Refresh retries failed metadata and schedules incremental organization; restoring clears inherited dismissal for merged/split items. No model inference or online geocoding is performed.'

for path, _, operation in OPERATIONS:
    if path == '/ai/photos/{photoId}/similar/page':
        operation['description'] += ' First request uses pageSize (default 24, maximum 60); subsequent requests send only cursor. Cursors bind the user/session, query vectors, profile and search settings. Expired or invalidated cursors return HTTP 409. Results sort by descending media score then ascending photo ID; a video appears once. The viewer has a separate 30-second per-media return cache.'
    elif path == '/ai/capabilities':
        operation['description'] = 'Read the authenticated user eligibility, active profile, coverage and supported image/video formats. Does not perform inference. Video limits describe library indexing; direct video query uploads are not supported.'
        operation['responses']['200']['description'] = 'Capabilities, including readiness and any waiting reason.'
