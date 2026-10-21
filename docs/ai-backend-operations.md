# AI 后端部署与操作

前端提供管理员及搜索页面；也可通过 `/docs` 的 AI Admin / AI Search 分组操作。新接口使用 `{t,msgCode}` 和准确 HTTP 状态；错误另含稳定的 `error` 字符串。

## 部署

默认全局关闭、新用户默认关闭，管理员无自动资格；启动容器不下载或加载模型。数据库增量升级至当前 head `0005_smart_organization`（含视频帧、OCR 和智能整理迁移）。已有部署先备份 PostgreSQL 和媒体。在 `.env` 设置 `COMPOSE_PROFILES`（`ai-cuda`、`ai-cpu` 或 `ai-mps`，只启用一个），之后各命令自动使用该 profile：

```sh
docker compose stop backend ai-scheduler ai-maintenance
docker compose build
docker compose run --rm migrate
docker compose up -d
```

`.env` 增加独立随机 `AI_INFERENCE_TOKEN`、`AI_CONTROL_TOKEN`，不提交凭据。Web 只有推理密钥，scheduler 有控制密钥，只有 maintenance/migrate 获得 migration 数据库凭据。容器 runtime 无数据库凭据、媒体挂载、主机端口或 Docker socket；MPS 原生运行时通过鉴权主机端口提供服务，见下文。

CPU 专用部署使用 `COMPOSE_PROFILES=ai-cpu`（启动 `ai-runtime-cpu`，网络别名仍为 `ai-runtime`）；CUDA 使用 `COMPOSE_PROFILES=ai-cuda`。

CPU/CUDA 各自使用 `photoly_backend/inference/{cpu,cuda}/uv.lock`。CPU 镜像不含 CUDA wheel、不申请 GPU；CUDA 镜像需 NVIDIA 容器支持。管理 API 选择 cpu/float32 或 cuda/bfloat16；原生 Apple Silicon 使用 mps/float32，见 MPS 部署章节。NPU 不开放。

默认项目 photoly 的模型缓存卷为 `photoly_ai_models`。首次 Enable AI（ai.enable）下载固定 revision，文本与图像预热都通过才 ready。加载期限 840 秒；慢网络可预先缓存后重试。Web 不安装 torch。

WSL/Docker 必须分配足够的内存；修改 WSL 内存上限需要安排重启窗口，会影响同机其他 WSL 工作负载。CUDA 镜像固定使用 PyTorch 2.8.0+cu128 与 Transformers 4.57.3；实际设备是否可用，以 runtime 检测和模型预热结果为准。

## 初次启用与 Swagger

正常登录后在 Swagger 授权 `HRD-Token`。每次修改前 GET `/admin/ai/settings` 获取最新 generation。

1. 在 AI Management 选择 Device（默认 CPU），必要时保存资源设置；为所需用户开启个人 AI 访问权限。
2. 点击 **Enable AI**。后端自动创建或复用默认 SigLIP2 Profile、下载固定版本、加载并预热、准备索引，然后自动激活。高级设置可以选择 Qwen；旧部署默认沿用当前模型。
3. 首次启用自动开启 Scheduler；之后保留管理员选择。搜索在模型和索引结构就绪后即可使用，返回已处理内容；媒体失败不会阻塞整个图库开放。
4. **AI Search**、**Smart Tag** 为独立功能开关；Tag 仍需用户授权和主动开启。**Pause Scheduler** 停止新后台任务，在线搜索继续使用已有索引。
5. **Pause AI** 关闭所有 AI 功能并释放模型进程，下载/加载期间也可暂停。缓存、向量和各开关偏好保留，恢复时无需重做已完成工作。

对应 API 为 POST `/admin/ai/operations`，例如 `{"command":"ai.enable","generation":0}`，可附加 `"provider":"qwen3_vl"`；暂停使用 `ai.pause`。每次提供新的 `Idempotency-Key`。202 表示受理，operation succeeded 表示目标状态已接受，实际启动完成以 GET `/admin/ai/status` 的 `startup.stage=ready` 为准。同幂等键和同 body 重试返回同一操作，改变 body 返回 409。

`startup` 返回 stage、lifecycleId、completed/total、unit、updatedAt、reason、online。下载显示实际完成文件数（包含缓存命中），不是字节百分比或总启动百分比；无法计量的准备、加载、预热阶段显示不定进度。图库索引及 Smart Tag 进度独立展示。失败后再次 Enable 重试，修复离线容器后目标状态自动恢复。普通阈值修改不会取消加载；暂停/模型切换以独立生命周期标识屏蔽旧结果。

高级 API 继续支持 model.load/unload/restart、scheduler.start/pause/resume/stop、maintenance.enter/exit、backfill、retry_failed、rebuild、activate、purge。手动模型或维护操作退出自动生命周期管理，保留原有显式激活的完整覆盖检查；日常管理无需使用这些操作。完全离线的容器仍由部署工具恢复。

## 切换与回退

高级设置选择模型后点击 **Apply model**，自动准备并激活目标模型；回退时选择原模型即可。一次只驻留一个模型，切换期间 AI 搜索暂不可用，普通图库继续工作。目标模型与索引结构就绪后立即开放已有结果，同时后台补齐剩余媒体，无需等待全量覆盖。

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

文本<=512字符，还要通过模型token限制（SigLIP64、Qwen模板总长8192）。图片<=20MiB、40MP：JPEG/JFIF/PNG/WebP/BMP/TIFF/GIF/ICO（含 Pillow 识别为 MPO 的 JPG，读取主图/首帧）；首帧、EXIF方向、白底透明合成、RGB。查询图片只在内存使用，不保存向量。图库大图超出首版限制也标unsupported；RAW/PSD/SVG不反复重试。图库视频使用下文的 FFmpeg 多帧索引；直接上传视频作为查询输入不受支持。

错误包括401 unauthenticated；403 ai_not_enabled/admin_required；400 invalid_*；413 image_too_large/image_too_many_pixels/text_token_limit；415 invalid_image/unsupported_image；409 generation_conflict/maintenance_required/profile_in_use/profile_changed；429 user_query_busy/runtime_busy；503 maintenance/runtime_offline/model_not_ready/out_of_memory；504 query_timeout/inference_timeout。覆盖不全是成功响应附带 coverage。

## 调度与资源限制管理 API

使用现有 `GET /admin/ai/settings` 和 `PATCH /admin/ai/settings`，管理员认证与 generation 规则不变。不需要新数据库 migration，旧配置自动补齐兼容值并保留自定义限制，不自动开启 AI。旧配置 ram_policy=free，新配置 ram_policy=percent；保存 ram_max_percent 切换策略。enabled 是总开关，新增 search_enabled 默认为 true；smart_tag_enabled 保持原值。

先 GET 获取实际 generation，再 PATCH，例如：

```json
{
  "generation": 0,
  "settings": {
    "background_interval_seconds": 10,
    "background_max_per_minute": 6,
    "cpu_pause_percent": 80,
    "gpu_pause_percent": 90,
    "ram_max_percent": 85
  }
}
```

| 字段 | 默认值 | 允许范围/含义 |
| --- | ---: | --- |
| background_interval_seconds | 2 | 0–3600；后台任务领取起点之间的最小秒数，0关闭额外间隔 |
| background_max_per_minute | 0 | 0–600；滚动60秒窗口最多领取次数，0表示不设该上限 |
| cpu_threads | 4 | 1–64；PyTorch CPU计算线程，也影响CUDA模式中的CPU计算 |
| cpu_pause_percent / cpu_resume_percent | 85 / 75 | 简化界面上限11–100；只提交上限时自动推导恢复值=上限−10。高级 API 仍支持显式合法配对 |
| gpu_pause_percent / gpu_resume_percent | 90 / 80 | 同 CPU；仅 CUDA 使用 GPU 利用率保护 |
| ram_max_percent | 85 | 6–100；有效系统/容器内存使用率，降低5个百分点后恢复 |
| memory_pause_free_mib / memory_resume_free_mib | 1024 / 2048 | 旧配置兼容门槛；保存 ram_max_percent 后改用百分比策略 |
| vram_pause_free_mib / vram_resume_free_mib | 1024 / 2048 | 可用显存门槛，单位MiB |
| disk_pause_free_mib / disk_resume_free_mib | 2048 / 3072 | 磁盘余量门槛，单位MiB |

所有字段必须是整数，布尔/小数/null不接受。MiB字段范围1–16777216，恢复值必须大于暂停值；只修改一侧时，也与当前另一侧一起校验。不合法数值返回400 invalid_resource_limits，不合法恢复/暂停关系返回400 invalid_resource_hysteresis；失败不改变generation。

间隔、每分钟上限、资源门槛在后续调度轮次生效，不重载模型、不打断已在途任务。控制循环仍每轮结束后等待2秒，较长限速不会阻塞管理操作或心跳。间隔是领取起点之间的最小距离，不保证精确按时启动。每分钟上限统计后台尝试，包括已领取后失败/取消的尝试和重试；额度/资源等待不占次数。

限速记录保存在ai_control_state的background_throttle对象，与任务领取及容量预留同事务。在全局控制锁内检查和登记，不会因并发领取、worker重启、删除照片/任务而绕过；事务回滚不消耗次数。在线查询不计入后台限速，仍遵守其自身队列与单用户并发限制。

修改cpu_threads前，先 Pause AI 并等待卸载，再保存，之后 Enable AI；底层 model.unload/load API 也继续支持。模型desired状态仍为loaded时修改线程数返回409 unload_before_thread_change。线程数属于部署配置，不改变profile fingerprint，不需要重算向量。

GET `/admin/ai/status` 新增backgroundLimits：lastStartedAt、startsLastMinute、nextAllowedAt、reason（interval_wait/rate_limited/null），以及只读maxConcurrency=1、batchSize=1。nextAllowedAt仅是限速允许的最早时刻，还要满足模型、资源、资格等条件。

这些门槛控制后台是否开始计算，不能将运行中的CPU/GPU使用率强制压到某百分比；在线查询和已加载模型仍可占用资源。CPU/RAM容器硬配额属于Compose部署配置。当前不开放batch或并发数调大，仍一次一张、单推理进程。

## 资源、容量与恢复

后台 batch 保守固定为1。CPU平滑、RAM/VRAM/磁盘硬门槛和暂停/恢复滞回优先；未知指标明确标记。CPU采样跨HTTP线程共享差值，报告host/container和cgroup容量，指标来自实际runtime容器。

默认预算10GiB：所有profile的向量/行/索引估算与实际关系占用。领取在全局短事务锁内预留，完成结算；失败、取消、租约过期回收。预算不是物理硬上限；WAL、备份、索引临时空间另留余量。普通删除不会立即缩小文件。首版准入汇总数据库使用量，超大图库需根据性能数据优化计数缓存。

任务租约120秒；scheduler每2秒续约并检查启停意图，maintenance每20秒续约，使用不可复用lease token。崩溃后最多等待租约到期；旧worker不能覆盖结果。重启恢复desired model/scheduler。资源不足、离线、超时/OOM暂态等待不消耗失败次数，其他失败最多5次并指数退避。状态不返回堆栈、凭据、内部路径或向量。

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

首次使用：在 Resources 选择 Device 并保存 → 在 User Management 授予所需用户资格 → Enable AI。模型与索引结构就绪后自动开放搜索，媒体索引在后台逐步补齐。注册和管理员新增账户使用 new-user 默认；更改默认不会修改存量用户。

页面提供 Enable/Pause AI、Enable/Pause Scheduler，以及即时保存的 AI Search/Smart Tag 开关。高级设置中的 Model 选择会在 Enable AI 或 Apply model 时自动处理 Profile、加载与激活；回退选择旧模型即可。下载文件进度、启动阶段、媒体索引进度和 Smart Tag 进度分开显示。

Resources 显示 CPU/GPU utilization、RAM max 和 Device，支持恢复推荐值；线程、速率、存储预算、磁盘/显存保护和相似度阈值折叠在 Advanced settings。设备和CPU线程要求先 Pause AI 再保存。暂停 Scheduler 不卸载模型，Pause AI 会释放模型资源且保留偏好。Diagnostics and operation history 展示服务、配额、操作历史。页面不能代替 Docker 启动已离线容器。

新接口GET `/admin/ai/capabilities`给出字段范围、单位、配对约束、生效方式和运行时设备可用性。可用不代表已验证；显示的verified仅代表当前运行时会话真实加载和双模态预热成功。NPU未支持。

操作返回202只表示受理。页面轮询queued/running/succeeded/failed/cancelled及真实观测状态；网络丢失时保留幂等请求并可恢复，刷新也会读取服务端进行中操作。设置冲突保留草稿，不自动覆盖其他管理员的新设置。jobs支持`order=desc&before=<id>`浏览最近操作，原升序after仍兼容。

Search模式选择器保留Natural language和旧字段搜索；自然语言支持目录/相册组合过滤，默认20条、最多100条，不分页。Image search上传入口已移除，原上传查询图API继续兼容。图库图片检索使用下述查看器推荐区。cosine排序不是匹配概率。

媒体查看器在图片或视频和Metadata下方增加divider和Scroll down to discover similar photos and videos提示。打开媒体不推理；继续下滑进入推荐区才加载首批24张，之后接近底部继续加载，无100张上限。也可用Load more键盘操作。点击缩略图在同一弹窗内切换，Back to previous photo 在离开不足 30 秒且推荐上下文仍有效时恢复上一项的推荐和滚动位置；超过 30 秒回到顶部并收起。持续查看不自动过期。右下角 Back to top 立即回顶、收起并删除当前项缓存；关闭后原图库状态不变。停止等待仅取消客户端请求，服务端可能仍在计算，立刻再次查询可能收到busy。

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

作用范围：图库图片和视频 `/ai/photos/{photoId}/similar`、其 `/page` 分页接口，以及上传查询图 `/ai/search/image`；自然语言 `/ai/search/text` 保持不变。阈值由服务器读取，调用者不能通过请求参数关闭。SQL 在 limit/分页之前过滤 `score >= threshold`，用户隔离、版本检查及排序保持不变。覆盖率仍表示索引完成情况，不是达到阈值的数量。

用户 capabilities 与搜索响应包含 `similarityThreshold`（自然语言响应为 null）。分页上下文绑定阈值，设置改变后旧游标返回 HTTP 409 / `search_settings_changed`。查看器在翻页或 capabilities 轮询时清空旧推荐与历史缓存并提示 **Reload recommendations**，不会拼接两套阈值的结果。轮询间隔沿用可见页面 15 秒、隐藏页面 30 秒；已展示页面不会瞬间同步其他管理员会话的更新。

## Apple Silicon：Mac 原生推理 + Docker 服务

Apple GPU 使用 PyTorch MPS，在 macOS 主机运行模型；Docker Desktop 内继续运行 Web、Flask、PostgreSQL、Redis、scheduler 和 maintenance。需要 macOS 14+ 和原生 ARM64 Python（不使用 Rosetta）、`uv` 和 Docker Compose 2.24.4+。运行时固定 Python 3.12、PyTorch 2.8.0、Transformers 4.57.3，独立环境包含 `app` 导入所需依赖；不会替换后端 `.venv`。

在仓库根目录执行：

```sh
sh photoly_backend/scripts/mps-runtime.sh install
sh photoly_backend/scripts/mps-runtime.sh check
sh photoly_backend/scripts/mps-runtime.sh start
```

先在 `.env` 配置现有 `AI_INFERENCE_TOKEN` 和 `AI_CONTROL_TOKEN`；主机与 Docker 必须使用同一组值。脚本只读取运行时相关键，环境变量优先，不执行 shell、不展开变量。可用 `--env-file /absolute/path/.env` 选择文件。`check` 验证设备及一次真实 MPS 运算，不下载模型。

在 `.env` 设置 `COMPOSE_PROFILES=ai-mps` 和 `AI_RUNTIME_URL=http://host.docker.internal:8090`，另开终端启动其余服务：

```sh
docker compose up -d --build
```

此 profile 不启动 Linux `ai-runtime`，也不请求 NVIDIA GPU。不要同时启用 `ai-cuda` 或 `ai-cpu`。后端和调度器连接 `AI_RUNTIME_URL`；调度器在运行时离线时保持控制循环，主机运行时恢复后重新连接。

原生服务默认监听 `0.0.0.0:8090`，仅供可信本机/Docker 网络使用，继续严格区分推理与控制令牌。可设置 `AI_RUNTIME_HOST` 和 `AI_RUNTIME_PORT`；混合部署必须保持 Docker 可访问的监听地址，且主机及 Compose 使用相同端口。默认模型缓存为 `~/.cache/photoly/models`，可用 `HF_HOME` 覆盖。令牌不会输出到日志。

管理页选择 **Apple Silicon GPU (MPS) / float32**，保存设置后显式加载模型。已有配置保持原设备；更改设备前需卸载模型。文字和图片预热均成功后才记录验证成功。仅启动服务不会下载模型权重；首次点击加载可能需要较长下载时间。

SigLIP2 和 Qwen3-VL 共用现有固定模型版本、预处理及向量指纹；切换设备无需重建索引。MPS 不自动降级 CPU，也不启用算子 CPU fallback。16 GB Mac 推荐先使用 SigLIP2；Qwen float32 权重及图片推理开销较高，是否可运行以实际加载和推理验证为准，不以设备可用代替模型可用。

MPS 使用主机统一内存的 RAM pause/resume 阈值；GPU 利用率、独立 VRAM 显示 Unknown，不参与 MPS 调度阻塞。CPU 和磁盘阈值仍生效。`out_of_memory` 表示内存不足，`device_unavailable` 表示设备不可用，`unsupported_operator` 表示算子不兼容；错误不会把失败的组合标成已验证。

停止前可在管理页暂停调度、卸载模型，再在原生终端按 Ctrl+C；Docker 服务可用同一 Compose 组合执行 `stop`。重新运行 `start` 即可恢复服务，模型生命周期继续由管理员配置控制。

独立真机验收（按需下载权重，不在普通 CI 运行）：

```sh
cd photoly_backend
HF_HOME="$HOME/.cache/photoly/models" PYTORCH_ENABLE_MPS_FALLBACK=0 PYTHONPATH=. \
  inference/mps/.venv/bin/python scripts/ai_model_smoke.py siglip2 mps \
  --samples tests/fixtures/ai --output /tmp/photoly-mps-results
# 同样命令将 siglip2 换成 qwen3_vl 验证第二个模型。
```

报告包含图文向量维度、归一化、图片自匹配、中英文检索、推理耗时、进程峰值 RSS，以及 MPS 卸载前后的分配量；统一内存不标作独立显存。`ai_service_acceptance.py` 同时支持 `--device mps --providers siglip2`，仅对隔离测试 Compose 数据库运行。

### 本次真机验证（2026-10-16）

环境：Apple M3、16 GiB 统一内存、macOS 26.6.2、PyTorch 2.8.0；关闭算子 CPU fallback。以下耗时来自仓库的 4 张固定测试图片和 8 条中英文查询，是小样本验收结果，不代表所有图库的性能。

SigLIP2 完成两次加载/推理/卸载。缓存命中时加载约 13.0 秒，图片推理中位数约 28 ms，文字约 8.5 ms；MPS 张量占用约 1.40 GiB，卸载后为 0。768 维向量的有限性、单位范数、图片自匹配及测试查询检索均通过。

隔离 PostgreSQL/Redis + Docker 后端连接原生 MPS 的验收通过：4 张图片后台索引完整生成，文字搜索、图片搜索、相似图片和 Smart Search 文字候选正常；原生运行时停止后返回 `503 runtime_offline`，重启后自动恢复并继续使用原有索引。测试服务和数据已清理。

Qwen3-VL Embedding 也完成两次图文推理及卸载。缓存命中时加载约 39.5 秒，图片推理中位数约 1.31 秒，文字约 0.11 秒；MPS 张量占用约 7.93 GiB，卸载后为 0。2048 维向量的有限性、单位范数和图片自匹配通过；8 条固定查询的 Recall@1 为 75%，Recall@3 为 100%。此结果证明本机可以运行两个模型，但不保证更大图片或更高内存压力下均可运行；16 GB 机器仍推荐 SigLIP2。

回归：411 项后端测试、112 项前端单元测试、96 项 AI 管理/Smart Search/相似图片浏览器测试通过；类型检查、ESLint、前端构建、Compose 合并配置及原生启动检查通过。浏览器覆盖 Chromium、WebKit 和移动端。

## Video frame indexing

Rebuild the backend, scheduler and maintenance images and run the migration service before starting them. Revision `0003_video_vectors` preserves image vectors and model fingerprints. Maintenance builds a second partial HNSW index per profile for video frames; existing image search stays available during this build. No native macOS decoding dependency is needed for the MPS hybrid deployment.

The backend/scheduler stream each library video into a private temporary snapshot, calculate its SHA-256, and sample 5%, 25%, 50%, 75% and 95% of its video-stream duration. Very short clips with at most five decoded frames use those frames. FFmpeg applies display rotation and scales down to a maximum edge of 1280 without upscaling; decoded images are RGB. A VFR tail with no frame after the sampling time contributes no invented duplicate. The [FFmpeg seek and rotation options](https://ffmpeg.org/ffmpeg.html) are applied during decoding. The shared probe/extraction deadline is 30 seconds; online requests retain their 60-second total deadline. Snapshot size uses `MAX_CONTENT_LENGTH` (default 600 MiB), independently of the 20 MiB image limit.

Only decoded frames reach the model runtime, serially. Frame vectors and the ready state commit atomically, with database constraints preventing incomplete frame sets. Video jobs reserve five vector charges; completed jobs charge actual frame count. Physical quota accounting includes the frame table and its indexes. Overwrite, deletion and profile purge cascade or invalidate frame rows. A replaced file cannot receive an old task's vectors. Temporary files are removed on success and failure. Decode timeouts retry at most five times; invalid/no-stream/no-frame inputs become unsupported with explicit job reasons. Use the existing failed-job retry operation after correcting transient decode problems.

Enabled users' old videos are picked up automatically by scheduler reconciliation, including videos previously marked unsupported by the image-only worker. Sampling version 1 is recorded before attempting decoding, so unsupported/failed videos do not enter an endless backfill loop. Rebuilding or changing the sampling version only requires new video vectors; image profiles are unchanged.

Search uses the highest cosine similarity across all query and document frames. Each video appears once; thresholding and seek pagination happen after media aggregation. Ordinary searches use exact scoring up to 5,000 eligible media items, then expanded per-frame ANN candidates and exact reranking; insufficient candidates fall back to exact scoring. Recommendation pagination is exact and caches the whole query vector set. SigLIP2 reuses stored video frames; Qwen regenerates frames with its query instruction once per recommendation context. Results expose an optional `matchedTimestampMs`; playback still starts at the beginning.

The viewer now offers “Similar photos and videos” for videos, with the same per-media 30-second return cache and Back to top/collapse action. Smart Search's existing semantic branch automatically includes videos. Supported extensions remain mp4, avi, wmv, mpeg, m4v, mov, flv, asf, f4v, rmvb, vob and rm; actual decoding depends on the installed FFmpeg codecs. There is no audio, subtitle or scene-cut analysis.

For opt-in real-model acceptance, run `scripts/ai_service_acceptance.py --device mps --video` inside the disposable test service configured to reach the native runtime (see the MPS section). This generates two five-scene videos from the checked-in image fixtures, indexes both alongside four photos, checks the middle-frame image match, mixed results, video queries, cursor pagination and Smart Search metadata. It does not use the production database or media directory.

Decoding accepts only the existing video container families and disables network protocols and playlist/image demuxers. This prevents uploaded playlists from referencing unrelated server files. A format extension alone is not treated as proof that the file contains decodable video.

### Video acceptance on Apple M3 / 16 GiB (2026-10-16)

The disposable PostgreSQL/Redis/backend/scheduler services called the native PyTorch 2.8 MPS runtime with two five-scene videos and four photos. Both profiles stored ten frame vectors, with half-precision stored L2 norms between 0.99986 and 1.00016. Image vectors were retained during profile/migration checks.

| Profile | Five-frame document inference per video* | Image → middle video frame | Video → mixed media and pagination |
| --- | --- | --- | --- |
| SigLIP2 / MPS float32 | 0.251–0.271 seconds | Passed | Passed; complete query exercise 0.486 seconds |
| Qwen3-VL-Embedding-2B / MPS float32 | 32.815–102.827 seconds | Passed with the matching letterboxed frame query | Hit the existing 60-second query deadline on this machine (HTTP 504 `query_timeout`) |

\* `duration_ms` measures serial frame inference, excluding snapshot and FFmpeg extraction. These are two local samples, not throughput guarantees. The host was under substantial memory pressure (about 12.4 GiB of swap in use); Qwen's initial full-library indexing also exceeded the acceptance runner's old 180-second wait. The video acceptance runner now allows up to 600 seconds for background completion, independently of the unchanged 60-second online deadline. No model precision, instructions, fingerprint or CPU fallback was changed to hide this limitation. SigLIP2 remains the recommendation for this 16 GiB machine.

An unletterboxed cat query initially ranked a coffee frame above the letterboxed cat frame in one Qwen video. Matching the query to the test video's framing recovered both middle-frame matches. Similarity reflects the chosen embedding model's ranking; it is not a guarantee of scene recognition. Deterministic database tests separately verify maximum-frame aggregation, all query-frame combinations, thresholds, stable ties, owner isolation and pagination independently of model relevance.

Final regression for this change: 430 backend tests passed against isolated PostgreSQL/Redis (five opt-in HTTP tests deselected); 113 frontend unit tests and 99 Chromium/WebKit/mobile tests passed. Type checking, ESLint, production build and `git diff --check` passed. Video tests include real FFmpeg decoding, rotation, VFR/short clips, corruption/no-stream/timeout, migration rollback/upgrade, frame completeness constraints, overwrite cancellation, retry limits, quota accounting, frame HNSW planning, exact reranking and Qwen query-vector cursor reuse. The real MPS results and limitation above are separate from these deterministic tests.

### Smart Tag organization

[Smart Tag deployment, rules, APIs and validation](smart-tag.md) describe the optional device tags and trip galleries. Deploy migration `0005_smart_organization` and rebuild the existing application images/frontend. Enable global AI, administrator Smart Tag, account AI access, and personal Smart Tag; then start the existing scheduler. No new service/container or native MPS dependency is required. Smart Tag uses offline GeoNames and already indexed document vectors; the model may be unloaded. Both new switches default off, and disabling preserves generated catalogs.


## Smart Search 与 OCR

默认 Smart Search 仅提供搜索框。`GET /photo/searchSmart?query=...` 无需 AI 资格，返回最多 100 个名称、Tag、Gallery 文字候选（不查询 Namespace）。前端另行请求 `/ai/search/text`，按等权 RRF `1/(60+rank)` 融合，按媒体 ID 去重并显示前 100 项；AI 不可用时保留文字结果。空查询清空结果，最多 512 字符，仅提交时执行语义搜索。

图片查看器的 Extract Text 使用 `/photo/ocr` GET/PUT 和 `/photo/ocr/recognize` POST。Tesseract 5 CPU 与 AI 开关及模型运行时独立；默认简繁中文、英文、日文。OCR 沿用 legacy HTTP 200 + msgCode 业务错误约定，与 `/ai/*` 的真实 HTTP 错误不同。详见 [OCR 部署与限制](ocr.md)。

本文各阶段的验证数字是当时快照，不代表每次修改都重新运行真实模型。Qwen 视频查询的 60 秒超时限制仍然适用；最终提交的完整验证结果见 PR 描述。
