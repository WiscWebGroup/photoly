# AI 后端部署与操作

前端提供管理员及搜索页面；也可通过 `/docs` 的 AI Admin / AI Search 分组操作。新接口使用 `{t,msgCode}` 和准确 HTTP 状态；错误另含稳定的 `error` 字符串。

## 部署

默认全局关闭、新用户默认关闭，管理员无自动资格；启动容器不下载或加载模型。数据库增量升级至 `0002_ai_backend`。已有部署先备份 PostgreSQL，再执行：

```sh
docker compose --profile ai build backend migrate ai-runtime ai-scheduler ai-maintenance
docker compose --profile ai up -d
```

`.env` 增加独立随机 `AI_INFERENCE_TOKEN`、`AI_CONTROL_TOKEN`，不提交凭据。Web 只有推理密钥，scheduler 有控制密钥，只有 maintenance/migrate 获得 migration 数据库凭据。runtime 无数据库凭据、媒体挂载、主机端口或 Docker socket。

CPU 专用部署（Compose >=2.24.4）：

```sh
docker compose -f docker-compose.yml -f compose.cpu.yml --profile ai build
docker compose -f docker-compose.yml -f compose.cpu.yml --profile ai up -d
```

CPU/CUDA 各自使用 `photoly_backend/inference/{cpu,cuda}/uv.lock`。CPU 镜像不含 CUDA wheel、不申请 GPU；CUDA 镜像需 NVIDIA 容器支持。管理 API 选择 cpu/float32 或 cuda/bfloat16。NPU 不开放。

默认项目 photoly 的模型缓存卷为 `photoly_ai_models`。首次显式 model.load 下载固定 revision，文本与图像预热都通过才 ready。加载期限 840 秒；慢网络可预先缓存后重试。Web 不安装 torch。

WSL/Docker 必须分配足够的内存；修改 WSL 内存上限需要安排重启窗口，会影响同机其他 WSL 工作负载。CUDA 镜像固定使用 PyTorch 2.8.0+cu128 与 Transformers 4.57.3；实际设备是否可用，以 runtime 检测和模型预热结果为准。

## 初次启用与 Swagger

正常登录后在 Swagger 授权 `HRD-Token`。每次修改前 GET `/admin/ai/settings` 获取最新 generation。

1. POST `/admin/ai/profiles`：`{"provider":"siglip2"}` 或 `qwen3_vl`。固定编码参数不可修改，重复创建返回同一 profile。
2. 轮询 profiles 等待 indexStatus=ready。
3. 模型未加载时可 PATCH settings：`{"generation":0,"settings":{"device":"cuda","precision":"bfloat16"}}`，替换实际 generation。
4. POST `/admin/ai/operations`：`{"command":"model.load","profileId":1,"generation":1}`，替换实际 ID/generation，设置新的 `Idempotency-Key`。
5. 轮询 operationId，等待 succeeded。202 仅表示受理。同幂等键和同 body 可重试，改变 body 返回 409。
6. PATCH settings 开启 enabled；PATCH `/admin/users/{userId}/ai` 开启所需用户。new_user_default 只影响之后创建的用户。
7. 提交 scheduler.start。存量回填每轮最多 4 位用户、每用户每 profile 最多 25 张，后台领取也按用户轮转。GET status 的 buildingCoverage 显示进度。
8. 当前受支持照片全部完成，索引与模型 ready 后，显式提交 activate。failed 阻止激活；unsupported 独立计数。

操作包括 model.load/unload/restart、scheduler.start/pause/resume/stop、maintenance.enter/exit、backfill、retry_failed、rebuild、activate、purge。pause/stop 停止领取 embedding，当前任务允许完成；控制循环常驻。完全离线的容器由部署工具恢复。

## 切换与回退

一次只驻留一个模型。跨模型流程：maintenance.enter → model.load 新 profile → scheduler.start/backfill → buildingCoverage 完成 → activate。维护期间 AI 搜索返回 503，普通图库继续工作。回退也显式维护、加载旧 profile；有新照片时先补齐旧 profile，再激活。

旧 profile 保留并计入容量。purge 拒绝活动、构建、已加载或仍被请求引用的 profile。maintenance 使用白名单 CREATE/DROP INDEX CONCURRENTLY、advisory lock、invalid index 检测及可恢复重试；不接受任意 SQL，清理保留 operation 审计。

## 搜索与前端接入

| 接口 | 输入/用途 |
| --- | --- |
| GET /ai/capabilities | 用户资格、全局开关、readiness、覆盖率与限制 |
| POST /ai/search/text | JSON text，topK 默认20，namespaceId/galleryId 可选 |
| POST /ai/search/image | multipart file，可选同样过滤字段 |
| POST /ai/photos/{photoId}/similar | JSON 可选过滤条件，排除自身 |
| GET /admin/ai/status | desired/observed、心跳、资源、配额、coverage/buildingCoverage |
| GET /admin/ai/users?ids=1,2 | 最多100个用户状态，避免前端 N+1 |
| GET /admin/ai/jobs | limit<=100，after 游标，kind/status 过滤 |

搜索返回 t={items:[{photo,score}],profileId,coverage,requestId}。photo 保持旧字段集合；score 是 cosine 相似度，不是概率，只在当前 profile 内解释。目录/相册必须归当前用户所有，SQL 中过滤所有权、profile、ready 和版本。

最多 topK=100，不支持 offset。候选<=5000精确查询；更大集合 iterative HNSW，结果不足且有剩余时间则回退精确查询。每用户最多一条在途查询，在线队列8，总查询期限60秒；在线优先，每最多8条给后台一次机会。

文本<=512字符，还要通过模型token限制（SigLIP64、Qwen模板总长8192）。图片<=20MiB、40MP：JPEG/JFIF/PNG/WebP/BMP/TIFF/GIF/ICO（含 Pillow 识别为 MPO 的 JPG，读取主图/首帧）；首帧、EXIF方向、白底透明合成、RGB。查询图片只在内存使用，不保存向量。图库大图超出首版限制也标unsupported；视频/RAW/PSD/SVG不反复重试。

错误包括401 unauthenticated；403 ai_not_enabled/admin_required；400 invalid_*；413 image_too_large/image_too_many_pixels/text_token_limit；415 invalid_image/unsupported_image；409 generation_conflict/maintenance_required/profile_in_use/profile_changed；429 user_query_busy/runtime_busy；503 maintenance/runtime_offline/model_not_ready/out_of_memory；504 query_timeout/inference_timeout。覆盖不全是成功响应附带 coverage。

## 调度与资源限制管理 API

使用现有 `GET /admin/ai/settings` 和 `PATCH /admin/ai/settings`，管理员认证与 generation 规则不变。不需要新数据库 migration，旧配置自动补齐默认值。

先 GET 获取实际 generation，再 PATCH，例如：

```json
{
  "generation": 0,
  "settings": {
    "background_interval_seconds": 10,
    "background_max_per_minute": 6,
    "cpu_pause_percent": 80,
    "cpu_resume_percent": 60,
    "memory_pause_free_mib": 2048,
    "memory_resume_free_mib": 3072
  }
}
```

| 字段 | 默认值 | 允许范围/含义 |
| --- | ---: | --- |
| background_interval_seconds | 2 | 0–3600；后台任务领取起点之间的最小秒数，0关闭额外间隔 |
| background_max_per_minute | 0 | 0–600；滚动60秒窗口最多领取次数，0表示不设该上限 |
| cpu_threads | 4 | 1–64；PyTorch CPU计算线程，也影响CUDA模式中的CPU计算 |
| cpu_pause_percent / cpu_resume_percent | 90 / 70 | 各1–100；恢复值必须小于暂停值 |
| gpu_pause_percent / gpu_resume_percent | 95 / 70 | 各1–100；恢复值必须小于暂停值 |
| memory_pause_free_mib / memory_resume_free_mib | 1024 / 2048 | 可用RAM门槛，单位MiB |
| vram_pause_free_mib / vram_resume_free_mib | 1024 / 2048 | 可用显存门槛，单位MiB |
| disk_pause_free_mib / disk_resume_free_mib | 2048 / 3072 | 磁盘余量门槛，单位MiB |

所有字段必须是整数，布尔/小数/null不接受。MiB字段范围1–16777216，恢复值必须大于暂停值；只修改一侧时，也与当前另一侧一起校验。不合法数值返回400 invalid_resource_limits，不合法恢复/暂停关系返回400 invalid_resource_hysteresis；失败不改变generation。

间隔、每分钟上限、资源门槛在后续调度轮次生效，不重载模型、不打断已在途任务。控制循环仍每轮结束后等待2秒，较长限速不会阻塞管理操作或心跳。间隔是领取起点之间的最小距离，不保证精确按时启动。每分钟上限统计后台尝试，包括已领取后失败/取消的尝试和重试；额度/资源等待不占次数。

限速记录保存在ai_control_state的background_throttle对象，与任务领取及容量预留同事务。在全局控制锁内检查和登记，不会因并发领取、worker重启、删除照片/任务而绕过；事务回滚不消耗次数。在线查询不计入后台限速，仍遵守其自身队列与单用户并发限制。

修改cpu_threads前，先提交model.unload并等待完成，再PATCH，之后model.load。模型desired状态仍为loaded时修改线程数返回409 unload_before_thread_change。线程数属于部署配置，不改变profile fingerprint，不需要重算向量。

GET `/admin/ai/status` 新增backgroundLimits：lastStartedAt、startsLastMinute、nextAllowedAt、reason（interval_wait/rate_limited/null），以及只读maxConcurrency=1、batchSize=1。nextAllowedAt仅是限速允许的最早时刻，还要满足模型、资源、资格等条件。

这些门槛控制后台是否开始计算，不能将运行中的CPU/GPU使用率强制压到某百分比；在线查询和已加载模型仍可占用资源。CPU/RAM容器硬配额属于Compose部署配置。当前不开放batch或并发数调大，仍一次一张、单推理进程。

## 资源、容量与恢复

后台 batch 保守固定为1。CPU平滑、RAM/VRAM/磁盘硬门槛和暂停/恢复滞回优先；未知指标明确标记。CPU采样跨HTTP线程共享差值，报告host/container和cgroup容量，指标来自实际runtime容器。

默认预算10GiB：所有profile的向量/行/索引估算与实际关系占用。领取在全局短事务锁内预留，完成结算；失败、取消、租约过期回收。预算不是物理硬上限；WAL、备份、索引临时空间另留余量。普通删除不会立即缩小文件。首版准入汇总数据库使用量，超大图库需根据性能数据优化计数缓存。

任务租约120秒，每20秒续约，使用不可复用lease token。崩溃后最多等待租约到期；旧worker不能覆盖结果。重启恢复desired model/scheduler。资源不足、离线、超时/OOM暂态等待不消耗失败次数，其他失败最多5次并指数退避。状态不返回堆栈、凭据、内部路径或向量。

关闭用户后拒绝搜索、取消任务、运行提交重验资格，旧向量保留。替换立即使旧向量失效，同事务登记新版本任务；标题/标签/目录变化不重算。原文件句柄在短用户锁内取得，复制/解码/推理在锁外执行。

## 测试与验收

默认CI不下载权重：

```sh
docker compose -p photoly-ci -f compose.test.yml build tests backend migrate smoke ai-runtime ai-scheduler ai-maintenance
docker compose -p photoly-ci -f compose.test.yml run --rm tests
docker compose -p photoly-ci -f compose.test.yml run --rm smoke
docker compose -p photoly-ci -f compose.test.yml down -v
```

smoke包含关闭后端的测试，只能用于隔离项目。真实模型单独运行 scripts/ai_model_smoke.py 和 scripts/ai_service_acceptance.py；后者URL固定为隔离http://backend:8084。compose.ai-real-test.yml显式替换runtime，只复用模型缓存，不连接生产数据库和媒体。测试输出写入本地 docs/verification，作为运行产物保留，不纳入源代码提交。

普通图库健康与AI readiness分离；前端按OpenAPI接入，不读取新增实体字段或向量。


## 浏览器管理与检索

管理员入口 `/thepersonincharge/ai`（侧栏 AI Management）。首页仪表盘现在显示 PostgreSQL、pgvector 和 schema revision。普通图库健康与AI状态独立。

首次使用：创建profile并等待index ready → 在Settings选择cuda/bfloat16或cpu/float32并保存 → Load model → 开启全局AI和User Management中指定用户资格 → Start scheduler → 等待当前受支持照片全部完成 → Activate profile。注册和管理员新增账户使用new-user默认；更改默认不会修改存量用户。

控制页提供步骤说明与高级操作。跨模型需在Advanced lifecycle controls中进入维护，再卸载旧模型并加载目标，完成构建后激活；激活会自动退出维护。回退同样需要维护和目标profile完整覆盖。只有不在使用的profile可清理；照片不随profile删除。

Settings中的间隔、速率、CPU/GPU及内存/显存/磁盘阈值在后续周期生效。设备、精度和CPU线程要求先卸载再保存、重新加载。并发与batch只读为1。暂停scheduler不卸载模型；彻底释放模型占用需Unload model。页面不能代替Docker启动已离线容器。

新接口GET `/admin/ai/capabilities`给出字段范围、单位、配对约束、生效方式和运行时设备可用性。可用不代表已验证；显示的verified仅代表当前运行时会话真实加载和双模态预热成功。NPU未支持。

操作返回202只表示受理。页面轮询queued/running/succeeded/failed/cancelled及真实观测状态；网络丢失时保留幂等请求并可恢复，刷新也会读取服务端进行中操作。设置冲突保留草稿，不自动覆盖其他管理员的新设置。jobs支持`order=desc&before=<id>`浏览最近操作，原升序after仍兼容。

Search模式选择器保留Natural language和旧字段搜索；自然语言支持目录/相册组合过滤，默认20条、最多100条，不分页。Image search上传入口已移除，原上传查询图API继续兼容。图库图片检索使用下述查看器推荐区。cosine排序不是匹配概率。

照片查看器在原图和Metadata下方增加divider和Scroll down to discover similar images提示。打开图片不推理；继续下滑进入推荐区才加载首批24张，之后接近底部继续加载，无100张上限。也可用Load more键盘操作。点击缩略图在同一弹窗内切换，Back to previous photo恢复上一张的推荐和滚动位置；关闭后原图库状态不变。停止等待仅取消客户端请求，服务端可能仍在计算，立刻再次查询可能收到busy。

### 前端与真实CUDA验收命令

前端使用packageManager固定的pnpm10.30.1。常规CI运行typecheck、lint、Vitest、build与Playwright，真实模型测试默认不运行、不下载权重。

```sh
# Repository root: disposable browser backend, only loopback port 18384.
docker compose -p photoly-phase3-real -f compose.test.yml -f compose.ai-real-test.yml -f compose.frontend-real-test.yml up -d --build backend ai-scheduler ai-maintenance
# photoly_frontend_vue directory (POSIX environment syntax shown):
PHOTOLY_REAL_AI=1 pnpm exec playwright test -c playwright.real.config.ts
# After restarting that same disposable stack:
PHOTOLY_REAL_RECOVERY=1 pnpm exec playwright test -c playwright.real.config.ts
# Repository root, only after completing verification:
docker compose -p photoly-phase3-real -f compose.test.yml -f compose.ai-real-test.yml -f compose.frontend-real-test.yml down -v
```

真实验收固定使用18384隔离API和5174前端，不提供生产URL选项。缓存卷是外部卷，清理测试环境不删除缓存。两种Vite测试模式使用独立缓存目录，避免并发开发服务器互相干扰。运行生成的数据与截图写入本地 docs/verification，不纳入源代码提交。


### 相似图片分页接口

`POST /ai/photos/{photoId}/similar/page`：首请求`{"pageSize":24}`（可省略，1–60）；续页仅传`{"cursor":"<opaque cursor>"}`。返回`items/profileId/coverage/requestId/nextCursor/hasMore`。没有总命中数，`hasMore=false`时结束。

首次复用兼容向量或调用一次query编码，续页复用Redis中的短期查询向量，不重复推理。Redis上下文有效期15分钟，绑定用户、登录会话、源照片版本和profile。游标不放路由或localStorage。过期返回409 cursor_expired，源图变化返回stale_content，profile切换返回profile_changed，前端显式重新加载；Redis重启后游标也会失效。

分页使用精确cosine距离和photo ID组合的seek排序，排除源照片并逐页重新过滤用户、ready及内容版本。既有文本/图片/相似top-k接口保持原策略与上限。没有新增数据库migration。此路径为稳定分页选择精确排序，大图库仍受查询超时保护；不承诺任意规模的固定延迟。浏览期间新增或更新图片需重新加载才能完整反映新的排序，不是长期数据库快照。

界面短期保留已加载结果与导航历史，退出/关闭后释放；后台资格撤销或profile变化会清除失效结果。缩略图懒加载，每页24项、单个在途请求；失败保留已有结果并提供Retry。加载按钮消失时恢复键盘焦点，Escape仍可关闭弹窗。

真实 CUDA 浏览器测试覆盖该滚动推荐交互，运行结果写入本地 docs/verification/similar-scroll。


## 相似图片阈值（2026-10-16）

AI Management → Settings and resource limits → **Similarity threshold (optional)**。
输入 -1～1 之间的数值后点击 **Save AI settings**。留空或点击 **No threshold** 后保存即取消限制；新部署和已有未配置的部署默认都是 `null`（不限制）。阈值使用 cosine 相似度，不能理解为匹配概率；不同模型的分数分布不同。

管理 API 沿用带 generation 的 `PATCH /admin/ai/settings`，例如：

```json
{"generation": 3, "settings": {"similarity_threshold": 0.75}}
```

取消限制传 `"similarity_threshold": null`；服务端拒绝布尔、字符串、非有限值和越界值。无需 migration、重建向量或重新加载模型。修改会持久化，使用现有管理员权限及 generation 冲突检查。

作用范围：图库图片 `/ai/photos/{photoId}/similar`、其 `/page` 分页接口，以及上传查询图 `/ai/search/image`；自然语言 `/ai/search/text` 保持不变。阈值由服务器读取，调用者不能通过请求参数关闭。SQL 在 limit/分页之前过滤 `score >= threshold`，用户隔离、版本检查及排序保持不变。覆盖率仍表示索引完成情况，不是达到阈值的数量。

用户 capabilities 与搜索响应包含 `similarityThreshold`（自然语言响应为 null）。分页上下文绑定阈值，设置改变后旧游标返回 HTTP 409 / `search_settings_changed`。查看器在翻页或 capabilities 轮询时清空旧推荐与历史缓存并提示 **Reload recommendations**，不会拼接两套阈值的结果。轮询间隔沿用可见页面 15 秒、隐藏页面 30 秒；已展示页面不会瞬间同步其他管理员会话的更新。
