# OpenCode 网关竞品扫描（2026-11）

> 扫描时间：2026-11-10  
> 范围：GitHub 上近 30 天有提交的 OpenCode Zen/Go 网关、同源分叉和相邻项目  
> 方法：GitHub Search API 检索 + 浅克隆源码/提交记录核对；未做运行时验证  
> 本地基线：`oc2api` HEAD `58f5667`（2026-10-30），与 `jasonxu114514/opencode2api` 同源

## 1. 结论

按“收益 / 成本”排序，最值得吸收的是：

1. **官方工具名回落 + 工具历史清洗**（上游 #166）：免费层会调用 `shell`、`read`、`edit` 等官方工具，现有响应映射只还原 `clint_` 前缀的客户端工具，客户端会收到 “No such tool available: shell” 或残留 ghost call。
2. **标题请求识别与兜底**（上游 #169）：OpenCode v2 的标题请求没有 tools、消息很少；当前 shaping 会强行注入官方工具，容易被 403/429，最终没有标题。
3. **免费额度分类（Nystic 思路，已否决全局门）**：`FreeUsageLimitError` 经上游源码核实按出口 IP/网段计费，不是全局共享桶；保留出口轮换与 `Retry-After` 冷却，不做全局快速失败。
4. **SSE 保活（已落地）+ 提交后空闲看门狗**（dengyie）：长推理期间没有心跳，中间层或客户端可能掐断；本次已加入默认 15 秒、仅在事件边界注入的注释保活；上游卡死的看门狗仍未实现。
5. **配额探测 + 墙前预检 + inflight 闸门**（dengyie）：当前只有通用冷却和 `Retry-After`，没有主动探测恢复、没有全池 wall 的零上游调用拦截，也没有并发准入控制。
6. **按 Key 绑定出口 / WARP 空池降级 / 代理会话模板**（上游 #170、enhance、6Km）：多 Key 场景下出口与配额/cache 的稳定性还不够；WARP 运行时池空会让 key tier 一起 503。

本次唯一落地的竞品能力是 SSE 保活；全局额度门验证为负收益后撤销。其余条目仍需逐项实测后再决定。

## 2. 活跃项目清单

Star 数为 2026-11-10 检索值；0 star 不代表不活跃。

| 项目 | Stars | 语言 | 最近提交 | 近期重点 | 与 oc2api 的关系 |
| --- | ---: | --- | --- | --- | --- |
| [jasonxu114514/opencode2api](https://github.com/jasonxu114514/opencode2api) | 1.4k | Go | 11-05 | 按 Key 代理绑定、被动可用性、标题请求、官方工具名 | 同源代码上游，可直接跟进 |
| [dengyie/opencode2api](https://github.com/dengyie/opencode2api) | 0 | Go | 11-10 | SSE 保活、空闲看门狗、配额探测、inflight、GEO、TetherPool | 同源分叉，免费层稳定性最完整 |
| [Userluckytian/opencode2api_enhance](https://github.com/Userluckytian/opencode2api_enhance) | 26 | Go/Tauri | 11-07 | WARP 空池自动降级、通道顺序、额度可视化、模型目录新鲜度 | 外部管理器，借鉴故障处理 |
| [6Kmfi6HP/opencode2api](https://github.com/6Kmfi6HP/opencode2api) | 155 | Go | 11-09 | CLI/TLS 指纹、`launch` 子命令、环境代理、多上游 | 独立实现，借鉴产品化 |
| [12errh/zen-proxy](https://github.com/12errh/zen-proxy) | 132 | JS | 11-09 | v2 免费层门禁、身份对、工具别名、BYOK、流式 stall | v2 门禁的参考实现 |
| [Nystic-Shadow/Opencode2api](https://github.com/Nystic-Shadow/Opencode2api) | 13 | JS | 11-01 | 额度门（假设已被推翻）、出口池、按 Key 队列、通知 | 控制面参考，跳过公共代理发现 |
| [TiaraBasori/OpenCode2API](https://github.com/TiaraBasori/OpenCode2API) | 430 | JS | 11-07 | 包装本地 OpenCode runtime、逐 Key 计量持久化 | 架构相反，只借计量持久化 |
| [IMROVOID/OpenCode2API](https://github.com/IMROVOID/OpenCode2API) | 1 | TS | 11-08 | Cloudflare Worker 部署、边缘身份轮换 | 部署形态不同，暂不吸收 |
| [caozhiyuan/copilot-api](https://github.com/caozhiyuan/copilot-api) | 1.1k | TS | 11-10 | Responses WebSocket 续接、SSE 失败分类 | 相邻项目，观察 |
| [Soju06/codex-lb](https://github.com/Soju06/codex-lb) | 3.4k | Python | 11-10 | 账号配额排序、多文件 auth 导入 | 相邻项目，只借排序思路 |

## 3. 同源上游：7 个提交可直接评估

已完成同步：本地从 `58f5667` 快进到 `b296a61`（`v1.4.7`），以下 7 个提交均已并入基线。表中“本地现状”保留同步前状态，便于对照。

| 提交 | 变更 | 本地现状 | 优先级 |
| --- | --- | --- | --- |
| [`87e7983`](https://github.com/jasonxu114514/opencode2api/commit/87e7983) | 恢复官方工具名，丢弃 ghost call，清洗工具历史 | 只做 `clint_` 客户端工具双向映射 | P0 |
| [`86be64b`](https://github.com/jasonxu114514/opencode2api/commit/86be64b) | 保留 v2 标题请求；WARP 429 不重复走已试出口 | 无标题识别；WARP 会重复尝试 | P0 |
| [`079cd7b`](https://github.com/jasonxu114514/opencode2api/commit/079cd7b) | 代理健康探测改为 OpenCode 源站 | 仍探测 `cloudflare.com` | P0 |
| [`cee32f7`](https://github.com/jasonxu114514/opencode2api/commit/cee32f7) | 被动可用性跟踪，主动探测改为 opt-in | 只有主动探测 | P2 |
| [`6fbe12c`](https://github.com/jasonxu114514/opencode2api/commit/6fbe12c) | 按 Key 绑定代理，失败时 fail-closed | 轮询分配，失败即迁移 | P1 |
| [`b296a61`](https://github.com/jasonxu114514/opencode2api/commit/b296a61) | Playground 请求固定到单个 relay | 诊断链路可后续跟进 | P3 |
| `46eb13e` | 合并批次后的构建整理 | 无需单独跟进 | — |

### 3.1 官方工具名回落（#166）

免费层请求会被补上官方工具集（`shell`、`grep`、`glob`、`read`、`task` 等），模型会照着系统提示调用它们。当前 [toolshape.go](C:/Users/Administrator/Desktop/oc2api/internal/gateway/toolshape.go:180) 只把客户端工具重命名成 `clint_…`，[tools.go](C:/Users/Administrator/Desktop/oc2api/internal/protocol/tools.go:27) 的 `ToolRenames` 也只有 client↔wire 两个方向；官方名没有映射回客户端声明的 `Bash`/`Read`/`Edit`，流式改写也只还原 wire 名。

上游新增了：

- `OfficialToolCandidates`：`shell`/`bash` → `Bash`，`read` → `Read`，`edit` → `Edit`，`task`/`subagent` → `Agent` 等；
- `bindOfficialToolAliases`：只在客户端确实声明了等价工具时绑定，避免乱猜；
- `sanitizeToolHistory`：删除未声明且无等价的工具调用及其配对的 `tool_result`/`tool_call_id`/`call_id`；
- `sanitizeToolChoice`：`tool_choice` 指向被删除工具时一并丢弃；
- 流式名称拆分保护：名称还是已知 wire 名前缀时先扣住参数，解析完再发，避免半截名称泄漏。

**价值**：直接消除 Claude Code/Cline 等客户端最常见的 “No such tool available” 和后续轮次模仿 ghost call 的问题。  
**成本**：M（约 300–500 行 + 测试）。

### 3.2 标题请求（#169）

OpenCode v2 的标题请求特征很明确：没有 tools、最多 3 条消息、没有 assistant/tool 轮次、带有固定的标题提示词。当前匿名链路会无条件 [`renameAndCompleteTools`](C:/Users/Administrator/Desktop/oc2api/internal/gateway/toolshape.go:180)，给标题请求补上完整工具集，结果被免费层以 403/429 拒绝，客户端拿不到标题。

上游新增 [title.go](https://github.com/jasonxu114514/opencode2api/blob/main/internal/gateway/title.go)（约 260 行）：

- `isTitleGenerationBody` / `isTitleGenerationRequest` 识别标题请求；
- 标题请求不注入工具、不追加官方工具集；
- 重试上限降为 1，避免在标题上烧配额；
- 429 或传输失败时本地合成标题（最多 6 词/40 字符）并按原协议返回。

**价值**：会话列表不再出现空白标题，同时少烧免费额度。  
**成本**：S–M。

### 3.3 探测目标（#153）

[refresh.go](C:/Users/Administrator/Desktop/oc2api/internal/gateway/refresh.go:15) 仍用 `https://cloudflare.com/cdn-cgi/trace` 判断代理健康。一个代理能到 Cloudflare，不代表能到 OpenCode；这会留下“健康但实际不可用”的代理。上游改为探测 OpenCode 源站，只有 16 行改动。

**价值**：减少假健康代理导致的请求失败。  
**成本**：XS。

### 3.4 被动可用性（#171）

本地 [availability.go](C:/Users/Administrator/Desktop/oc2api/internal/models/availability.go) 只有主动探测结果。上游新增 `RecordPassiveObservation`：

- 2xx 立即标记可用；
- 只有明确的 `model_not_found` / `model_retired` / `unknown model` 等权威错误才禁用；
- 普通 400/403/429/5xx/超时不禁用模型；
- 新增 `models.active_probes`，默认 `false`，用真实流量代替合成探测。

**价值**：省掉每小时一次的探测请求和额度消耗；模型下架/恢复反应更快。  
**成本**：M；需要和现有主动探测、WARP 探测策略一起设计，避免两套状态互相覆盖。

### 3.5 按 Key 绑定代理（#170）

当前 [nodePool](C:/Users/Administrator/Desktop/oc2api/internal/gateway/pool.go:624) 按 `i % len(proxies)` 轮询分配；代理失败时 [`RebindProxy`](C:/Users/Administrator/Desktop/oc2api/internal/gateway/pool.go:703) 会把 Key 迁到其他出口。对“免费额度按出口 IP 记账”的场景，这会让 Key 的配额和 prompt cache 一起漂移。

上游新增 `proxy_bindings`（Key 指纹 → 代理）：

- 绑定 Key 永远走指定代理，代理不可用时 fail-closed，不迁移、不直连；
- 代理恢复后立即恢复流量；
- 配置校验会拒绝未知 Key/代理，reload 失败时保留旧运行时；
- 只影响显式绑定的 Key，未绑定 Key、匿名链路、会话亲和不变。

**价值**：多 Key、多出口部署下稳定配额与缓存命中。  
**成本**：M。

## 4. dengyie 分叉：免费层稳定性最完整

[dengyie/opencode2api](https://github.com/dengyie/opencode2api) 以本地同一基线（v1.4.6 / `58f5667`）为起点，v1.5.0–v1.6.1 补了一整套生产化能力，CHANGELOG 写得很细。

### 4.1 值得吸收

| 机制 | 关键实现 | 配置 | 价值 |
| --- | --- | --- | --- |
| SSE 保活 | `internal/protocol/keepalive.go`：单写者串行化，Chat/Responses 发 `: keepalive`，Anthropic 发 `event: ping` | `performance.heartbeat_seconds`，默认 15 | 长推理不被中间层/客户端 idle 超时掐断 |
| 提交后空闲看门狗 | 上游无输出超过阈值 → 写 `upstream_idle_timeout` 错误帧 → `OnIdle` 取消上游 | `performance.idle_timeout_seconds`，默认 0（opt-in） | 卡死流不再耗完整个请求预算 |
| 配额冷却探测 | `quotaprobe.go`：每个 quota-cooled Key 每 N 秒发一次最小真实请求；单飞，成功后立即清冷却 | `performance.quota_probe_seconds`，默认 0（opt-in） | 额度窗口重开不必等第一个真实请求来发现 |
| 墙前预检 | 匿名 lane 10 分钟内 2 次 429 且存在可用 Key → 出口 walk 限 1 次；全池 wall 时预检直接拦截 | 与 `anonymous_backoff` 配合 | wall 期间每条请求从 3 次上游调用降到 0 次 |
| inflight 闸门 | `inflight.go`：全局限并发 + 可选排队；记录 `queued_total`/`inflight_queued` | `max_inflight`（0 关闭）、`queue_wait_seconds` | 防客户端 fan-out 瞬间打光额度 |
| GEO 分类 | 区域不可用 404 时停止同层 Key 轮换 | 内置 | 少烧 Key、错误更准确 |
| 可观测性 | `key_quota_cooled`、`quota_probe_skipped`、`upstream_body_read_failed` | 日志事件 | 全池被 wall 时能立刻看见，而不是等下一个调用者踩坑 |

关键设计点：**看门狗测的是上游输出，不是下游写入**，所以 keepalive 不会重置 idle 计时；数据写入、keepalive、错误帧共用一把锁，避免 SSE 帧交错。这个细节值得直接照搬。

### 4.2 不建议直接吸收

- **TetherPool 控制面**：把每个 Zen Key 绑定到外部 IP 池的 REST 控制面，属于商业代理集成，给核心网关引入外部依赖；除非明确要做节点池托管，否则不进入主线。
- **默认开启激进预检**：dengyie 的 wall 预检依赖“匿名 429 是出口级”的假设；上线前应对当前上游行为做一次实测，先以 opt-in 方式提供。

## 5. enhance：WARP 空池降级

[opencode2api_enhance](https://github.com/Userluckytian/opencode2api_enhance) 是包在引擎外面的 Tauri 管理器，v2.0.4 记录了一个很具体的故障：

- WARP 开启但池里 `active == 0`（常见于没有可用 IPv6、出口反复被判 duplicate egress）；
- 引擎把“Key 请求必须走 WARP”判断成硬条件，拿不到出口就直接放弃整个 Key tier；
- 结果是 10 个 Key 一个都用不上，客户端全部 `503 WARP pool has no ready egress`。

本地代码有同样的失败路径：[usesWARP](C:/Users/Administrator/Desktop/oc2api/internal/gateway/warp.go:37) 只看配置和模型，不看池里有没有可用出口；[doWARPAttempt](C:/Users/Administrator/Desktop/oc2api/internal/gateway/warp.go:156) 拿不到 lease 返回 `errWARPUnavailable`；[doKeyUpstream](C:/Users/Administrator/Desktop/oc2api/internal/gateway/upstream.go:1002) 直接 `break`，最终 [gateway.go](C:/Users/Administrator/Desktop/oc2api/internal/gateway/gateway.go:360) 返回 503。

启动时确实是 fail-closed（配置启用 WARP 但无法建立出口会被拒绝），但**运行期池子被冷却/注册失败清空**时仍会出现这个 503。

**建议**：

- 增加 `warp.fallback_to_proxies`（默认 true）或运行期降级状态机；
- `Snapshot().Active == 0` 或连续 N 次 `Acquire` 失败后，标记 WARP degraded，Keyed Zen 改走配置的代理/直连；
- `/healthz` 暴露 `warp_fallback`（原因、时间、strikes），WebUI 提供“重试 WARP”；
- 降级只做一次，不自动反复重试，避免继续烧出口；
- 匿名 lane 是否也降级要单独决定：匿名免费层可能按出口 IP 限额，降级到代理不一定能成功，但至少不应让 Keyed 请求一起 503。

其他可借鉴项：

- 通道顺序（WARP 优先 / Key 优先）：本地已有 `anonymous` + `prefer`，不需要新概念；
- 429 的 `Retry-After` 可视化：本地只对 Go 订阅配额做了窗口展示，免费 Key 的 reset 时间还没有；
- 模型目录启动期“前密后疏”刷新：本地固定 300 秒，若用户反馈新模型出现慢，可加启动期 10 秒刷新 + 读时陈旧后台刷新；
- 契约测试：本地发布了“去掉 `_test.go`”的版本，长期看应至少保留配置字段/wire 形状的黄金测试。

## 6. 6Kmfi6HP：产品化与指纹

[6Kmfi6HP/opencode2api](https://github.com/6Kmfi6HP/opencode2api) 是另一套 Go 实现，近期主要在做 CLI/TLS 指纹和启动体验。

### 6.1 已有、无需重复

- uTLS / HTTP2 / TLS hello / Bun runtime 指纹：本地已有 [`httpx/bun`](C:/Users/Administrator/Desktop/oc2api/internal/httpx/bun.go) 和 [`identity`](C:/Users/Administrator/Desktop/oc2api/internal/identity/request.go)；
- 请求体 harness preamble + 官方工具 manifest：本地已有 [`opencode_prompt.go`](C:/Users/Administrator/Desktop/oc2api/internal/gateway/opencode_prompt.go:98) 和 [`toolshape.go`](C:/Users/Administrator/Desktop/oc2api/internal/gateway/toolshape.go:180)。6Km 的注释可以作为“preamble 必须完整、不能截断”的回归参考，但不是新能力。

### 6.2 值得吸收

1. **`launch claude` / `launch codex` 子命令**（`internal/app/launch*.go`，约 25 KB + 测试）
   - 启动仅监听 localhost 的代理，然后用临时环境变量/`-c` 覆盖拉起子 CLI；
   - 不修改用户 `config.json`，退出即停；
   - 支持 `--model`、`--` 透传、Claude 的 `[1m]` 后缀与 `CLAUDE_CODE_AUTO_COMPACT_WINDOW`、Codex 的 custom provider 覆盖；
   - 价值：把“配网关”变成一条命令，明显降低上手成本；成本 L，属于产品功能而非网关核心。
2. **标准环境代理 + 自定义 CA**
   - `HTTPS_PROXY` / `HTTP_PROXY` / `NO_PROXY` 优先级，`OPENCODE2API_TLS_CA_FILE` / `TLS_INSECURE`；
   - 本地只有显式 `proxies` 配置，容器/企业网络下不够顺手；成本 S。
3. **`max_tokens_cap` / `max_tokens_cap_per_model`**
   - 防止客户端请求超大 `max_tokens` 浪费额度；成本 XS。
4. **多 `upstream_base_urls` + 会话亲和**
   - 多个反代/入口按 session 粘住一个 `(baseURL, proxy)` 对，兼顾负载与 prompt cache；
   - 本地 `UpstreamConfig` 是单值，若有反代/多区域需求再做；成本 M，暂列 P2。
5. **`socks5_sticky` 的量化结论**
   - 固定出口时 prompt cache 命中约 99.8%，随机轮换约 0%；
   - 本地已有 session 亲和（`CursorFor(ids.Session)`），但建议确认代理层是否也按 session 粘住，避免“逻辑粘、出口漂”。

## 7. zen-proxy：v2 门禁的参考实现

[zen-proxy](https://github.com/12errh/zen-proxy) 是单文件 JS 实现，近期把 OpenCode v2 免费层门禁摸得很透。

### 7.1 本地已覆盖

- 完整 harness preamble + 官方工具 + `stream: true`：本地已注入；
- 会话/项目身份对：本地 `identity` 已按进程加盐，并按调用方 scope 派生；
- 工具别名：本地用 `clint_<digest>` 重命名客户端工具，思路等价；
- BYOK 升级：本地已有匿名 → Key tier 的回退。

### 7.2 值得吸收

- **“门禁看的是 system prompt，不只是 tools”**：preamble 被截断到 ~1.8 KB 或换成通用提示词都会 429。本地注入的是完整捕获文本，建议加一个字节级回归测试，防止未来编辑时截断/改写。
- **只在“确定是免费额度耗尽”时升级 BYOK**：`exhaustedFreeVerdict` 区分额度耗尽与普通 429；本地 tier 回退要避免把限流/临时 429 当成额度耗尽反复换 Key。
- **流式 stall 的诚实提示**：上游长时间无输出时给客户端明确的 stall/错误文本，而不是静默等待；与 dengyie 的 idle watchdog 是同一类能力。
- **安全审计修复**：凭据、流、配置路径的边界检查；本地可对照做一次审计，但不是新功能。

## 8. Nystic：额度门假设与按 Key 队列

[Nystic-Shadow/Opencode2api](https://github.com/Nystic-Shadow/Opencode2api) 的 `server/upstreamQuota.js` 认为 OpenCode 匿名免费层是**共享桶**：`FreeUsageLimitError` 带同一个 `Retry-After`，换出口 IP 不能恢复容量，因此命中后全局快速失败。

**该假设已被上游权威源码否定（2026-11-10 核实）**：

- 仓库 `ai4nucleome/BioMaster`（OpenCode console 镜像）分支 `v2.0-alpha1` 的 `packages/console/app/src/routes/zen/util/ipRateLimiter.ts` 按 `ip` 计数；IPv6 取前 4 段（`/64`），IPv4 精确到地址。
- `lifetimeKey = buildRateLimitKey("ip", ip)`，`dailyKey` 再带 UTC 日期；默认模型 lifetime 上限是 daily 的 7 倍，新 IP 日限额翻倍。
- 超限抛 `FreeUsageLimitError`，`Retry-After` 到次日 UTC；换出口/换 IP 即可重新获得额度，WARP 与代理轮换正是有效恢复手段。

本地在 `b296a61` 上实现过全局门并做对照测试：命中后确实把上游调用从 3 次降到 1 次，但同一网关内所有出口/Key 被一并阻断，轮换恢复失效；匿名出口与 Key 原本已按出口冷却并遵守 `Retry-After`（[pool.go](C:/Users/Administrator/Desktop/oc2api/internal/gateway/pool.go:435)），全局门没有增量收益。**结论：不吸收，已撤销。**

保留的做法：`FreeUsageLimitError` 至多做日志/指标分类；匿名出口和 Key 继续走现有 `MarkFailure` 冷却，`#pool` 节点继续换 IP；响应透传真实 429 + `Retry-After`。

`server/requestQueue.js` 还提供了按 Key 的并发上限（`maxConcurrent`、`maxPerKey`、`maxQueue`、`maxWait`），比单一全局闸门更细。可以合并进 §4 的 inflight 设计。

`server/requestQueue.js` 还提供了按 Key 的并发上限（`maxConcurrent`、`maxPerKey`、`maxQueue`、`maxWait`），比单一全局闸门更细。可以合并进 §4 的 inflight 设计。

不建议吸收：自动发现/验证公共 HTTP 代理池。法律、可靠性和声誉风险都高，且与本地 WARP + 显式代理的产品取向不一致。

## 9. Tiara：只借计量持久化

[TiaraBasori/OpenCode2API](https://github.com/TiaraBasori/OpenCode2API) 包装的是本地 `opencode serve` 运行时，不是直连 Zen HTTP，架构与本地相反，不应整体照搬。

值得看的只有：

- **逐 Key 用量持久化**：`src/usage-persistence.js` 用临时文件 + rename 原子替换，默认每 5 分钟写一次，关闭时 flush；本地 [`APIKeyUsage`](C:/Users/Administrator/Desktop/oc2api/internal/admin/api_keys.go:32) 是纯内存，重启归零。若要把“每个本地 Key 用了多少”当账本，应加持久化。
- `cached_tokens`：本地已输出，不需要。
- 原生工具透传/会话亲和：本地已有。
- 客户端断开后停止后端工作：本地靠 context 取消，建议对非流式路径再核对一次。

## 10. 相邻项目观察

- [copilot-api](https://github.com/caozhiyuan/copilot-api)：Responses WebSocket 续接、SSE 协议失败分类、token 记账、共享刷新退避。WebSocket 是战略项，工作量大；当前 HTTP SSE 已够用，暂不进入排期。
- [codex-lb](https://github.com/Soju06/codex-lb)：按状态/剩余额度排序账号、多文件 auth 导入、UA→客户端家族映射。只有“按剩余额度排序 Key”适合本地 WebUI，优先级低。
- [IMROVOID/OpenCode2API](https://github.com/IMROVOID/OpenCode2API)：Cloudflare Worker 部署 + 边缘身份轮换。与 Go/WARP 架构冲突，暂不吸收。
- `OmniRoute` 一类通用网关： provider 数量、token 压缩不是 OpenCode 免费层的核心矛盾，未验证前不建议引入。

## 11. 差距矩阵

| 能力 | 本地现状 | 竞品来源 | 建议 | 优先级 / 成本 |
| --- | --- | --- | --- | --- |
| SSE keepalive | 已落地：默认 15s，事件边界注释帧 | dengyie | 保持；Anthropic 也用 SSE 注释帧，未单独发 `event: ping` | 已完成 / — |
| 提交后 idle watchdog | 无 | dengyie | 上游静默超时 → 错误帧 + 取消上游 | P1 / M |
| 配额冷却探测 | 只有通用冷却 | dengyie | `quota_probe_seconds` + 单飞探测 + 成功清冷却 | P1 / M |
| 墙前预检 | 无 | dengyie | 经验 wall 信号 + 预检拦截；先 opt-in | P1 / M |
| inflight 闸门 | 无 | dengyie / Nystic | 全局 + 按 Key 并发、排队、429 + Retry-After | P1 / S–M |
| 官方工具名回落 | 无 | 上游 #166 | 官方名→客户端等价工具 + 历史清洗 + 流式保护 | P0 / M |
| 标题请求 | 无识别 | 上游 #169 | 跳过 shaping、限 1 次重试、429 合成标题 | P0 / S–M |
| 全局免费额度门 | 无显式分类 | Nystic | **已否决**：额度按出口 IP 计，全局门会阻断轮换恢复 | — |
| 探测目标 | `cloudflare.com` | 上游 #153 | 改探 OpenCode 源站 | P0 / XS |
| 按 Key 代理绑定 | 轮询 + 失败迁移 | 上游 #170 | `proxy_bindings` fail-closed + 校验 + 脱敏 | P1 / M |
| WARP 空池降级 | 运行期 503 | enhance | `fallback_to_proxies` + 状态机 + health 展示 | P1 / M |
| 代理会话模板 | 无 | 6Km / Nystic | SOCKS5 用户名支持 `{{session-id}}` | P1 / S |
| 被动可用性 | 仅主动探测 | 上游 #171 | 真实流量观察 + `active_probes` opt-in | P2 / M |
| 逐 Key 用量持久化 | 内存 | Tiara | 原子快照 + 定时/退出 flush | P2 / S |
| `launch` 子命令 | 无 | 6Km | localhost 代理 + 子 CLI 环境隔离 | P2 / L |
| `max_tokens` 上限 | 无 | 6Km | 全局/按模型 cap | P2 / XS |
| 多上游 base URL | 单值 | 6Km | 多入口 + session 亲和 | P3 / M |

## 12. 建议落地顺序

### Sprint 1：先修“用户能感知”的错误（1–2 天）

1. [refresh.go](C:/Users/Administrator/Desktop/oc2api/internal/gateway/refresh.go:15) 探测目标改为 OpenCode 源站。
2. 标题请求识别：不注入工具、重试上限 1、429 合成标题。
3. 官方工具名回落 + 历史清洗 + 流式名称保护。
4. `FreeUsageLimitError` 仅做分类和日志；保留出口轮换与 `Retry-After` 冷却，不设全局门。

验收：

- 代理健康状态与真实上游可达性一致；
- 标题请求不再被工具 shaping 破坏，429 时仍有标题；
- 客户端声明 `Bash`/`Read`/`Edit` 时，官方 `shell`/`read`/`edit` 能正确映射；未声明的官方调用被丢弃，且不留下孤立 tool result；
- 额度 wall 不阻断其他出口：换出口/IP 仍可恢复；429 透传真实 `Retry-After`。

### Sprint 2：免费层稳定性（3–5 天）

1. SSE keepalive 已完成（默认 `performance.stream_keepalive_seconds=15`，事件边界注入）；剩余 idle watchdog。
2. 配额冷却探测 + 墙前预检（opt-in）。
3. 全局/按 Key inflight 闸门 + 队列指标。

验收：

- 60 秒无输出的推理流仍保持连接，客户端能收到 keepalive；
- 上游卡死时在 `idle_timeout_seconds` 内收到 `upstream_idle_timeout` 并释放上游；
- quota-cooled Key 在窗口重开后由探测恢复，不必等真实请求；
- 全池 wall 时预检拦截，wall 期间上游调用数接近 0；
- `max_inflight` 打满时请求排队或返回 429 + `Retry-After`，`queued_total` 可见。

### Sprint 3：出口与配额稳定性（约 1 周）

1. `proxy_bindings` 按 Key 绑定、fail-closed。
2. WARP 运行期空池降级 + 健康状态。
3. SOCKS5 `{{session-id}}` 模板。
4. 被动可用性跟踪（可与主动探测并存，先只记录不改变路由）。

验收：

- 绑定 Key 不因代理故障迁移/直连，配置 reload 失败保留旧运行时；
- WARP 池空时 Keyed Zen 可降级到代理，health 显示 degraded，不再全 Key 503；
- 同一会话的 SOCKS5 用户名稳定展开，prompt cache 命中可观测；
- 真实流量的 2xx / 权威 model-unavailable 能更新可用性，合成探测可关闭。

### 后续（按需求）

逐 Key 用量持久化 → `max_tokens` cap → 免费 Key reset 时间展示/告警 → `launch` 子命令 → 多上游 base URL。

## 13. 不建议照搬

| 项 | 原因 |
| --- | --- |
| TetherPool 外部控制面 | 引入外部依赖和部署复杂度；核心网关应保持自包含 |
| 公共代理自动发现（Nystic） | 法律、可靠性、声誉风险；与 WARP/显式代理的产品取向冲突 |
| 包装本地 OpenCode runtime（Tiara） | 架构相反，资源占用和故障面更大 |
| Responses WebSocket（copilot-api） | 工作量大；当前 HTTP SSE 已覆盖主流客户端，先观察 Codex 生态 |
| Codex 账号 LB / 多文件 auth（codex-lb） | 上游账号模型不同，不能直接套用 |
| 通用 token 压缩（OmniRoute 类） | 与本项目主要矛盾（免费层配额/稳定性）不匹配，且缺少本地实测 |

## 14. 方法与限制

- GitHub Search API 查询：`opencode zen`、`opencode gateway`、`opencode proxy`、`opencode2api`，按 `updated` 排序，筛选近 7 天有 push 的仓库。
- 对 10 个仓库做了 `--depth 60` 浅克隆，读取 README、CHANGELOG、提交 diff 和关键源码；提交数统计为 2026-10-15 以来。
- GitHub Core API 配额在检索时耗尽，仓库元数据来自 Search API；Star 数随时间变化。
- 未对上游发起真实推理流量；但“免费额度按出口 IP/网段计”的结论已查到 OpenCode console 源码（`ipRateLimiter.ts`、`error.ts`），并据此否决了 Nystic 的全局额度门。WARP 出口级配额与 429 行为仍建议上线前实测。
- 本地基线最初为 `58f5667`（2026-10-30），现已快进到上游 `b296a61`（`v1.4.7`，2026-11-05）。

## 15. 参考链接

- 上游仓库：https://github.com/jasonxu114514/opencode2api
- 上游关键提交：
  - #166 工具名回落：https://github.com/jasonxu114514/opencode2api/commit/87e7983
  - #169 标题请求：https://github.com/jasonxu114514/opencode2api/commit/86be64b
  - #170 按 Key 代理绑定：https://github.com/jasonxu114514/opencode2api/commit/6fbe12c
  - #171 被动可用性：https://github.com/jasonxu114514/opencode2api/commit/cee32f7
  - #153 探测目标：https://github.com/jasonxu114514/opencode2api/commit/079cd7b
- dengyie 分叉：https://github.com/dengyie/opencode2api/blob/main/CHANGELOG.md
- enhance 分叉：https://github.com/Userluckytian/opencode2api_enhance/blob/main/CHANGELOG.md
- 6Kmfi6HP 实现：https://github.com/6Kmfi6HP/opencode2api
- zen-proxy：https://github.com/12errh/zen-proxy
- Nystic 额度门：https://github.com/Nystic-Shadow/Opencode2api/blob/main/server/upstreamQuota.js
- OpenCode 额度源码：https://github.com/ai4nucleome/BioMaster/blob/v2.0-alpha1/packages/console/app/src/routes/zen/util/ipRateLimiter.ts
- OpenCode 额度错误：https://github.com/ai4nucleome/BioMaster/blob/v2.0-alpha1/packages/console/app/src/routes/zen/util/error.ts
- Tiara 计量持久化：https://github.com/TiaraBasori/OpenCode2API/blob/main/src/usage-persistence.js

## 16. 本次落地与验证记录（2026-11-10）

**上游同步**

- `58f5667` → `b296a61`（`v1.4.7`），快进合并 7 个提交，25 个文件变更（+1380/−126），无冲突。
- VPS 原本已是 `v1.4.7`，同步阶段无需重新部署。

**已落地：SSE 空闲保活**

- `internal/protocol/keepalive.go`：包装 `io.Reader`，空闲超过间隔后写入 `: keepalive\n\n`。
- 只在 `\n\n` / `\r\n\r\n` 事件边界注入；半截事件期间不写入，避免破坏 `data:` 行。
- 配置项 `performance.stream_keepalive_seconds` 默认 `15`，范围 `0–86400`，`0` 关闭。
- 在 `RenameStreamReader` 之后包装，同协议转发与转码流都覆盖。

**量化结果（400ms 上游静默）**

| 场景 | 首字节延迟 | 说明 |
| --- | ---: | --- |
| 原始流 | `400.9351ms` / 复跑 `401.0714ms` | 只能等到第二个事件 |
| 保活包装流 | `51.0562ms` / 复跑 `50.5971ms` | 收到 keepalive |
| `httptest` + `ForwardStream` | `50.3783ms` | 注释帧真实刷到 HTTP 客户端 |

400ms 空闲期内观察到 7 个 keepalive；半事件测试在 200ms 内不注入，补全事件后正常交付。保活相关测试 `-count=10` 全部通过。

**验证命令结果**

- `go test ./... -count=1`：通过（`internal/protocol` 有测试，其余无测试文件）。
- `go test ./internal/protocol -run "KeepAlive|ForwardStream" -count=10`：通过。
- `go vet ./...`：通过。
- `go build -trimpath ./cmd/opencode2api`：通过，产物 20,586,496 字节。
- `go test -race`：未能运行，环境无 cgo/gcc（`-race requires cgo`），不能声称 race 已验证。

**已否决：全局免费额度快速失败**

- 对照测试确实把首次请求的上游调用从 3 次降到 1 次、第二次降到 0 次；但全局状态会同时阻断所有出口和 Key，使换 IP 恢复失效。
- 上游 `ipRateLimiter.ts` 证明额度按出口 IP/网段计，见 §8；现有 `MarkFailure` 已按出口冷却并遵守 `Retry-After`，不存在需要全局门覆盖的共享桶。
- 相关生产代码和测试已删除；工作树只保留保活。

**提交**

- `1d1aade feat(stream): keep SSE responses alive while upstream is idle`
- `a4b641c test(protocol): cover SSE keepalive timing and event boundaries`
- 上游刻意维护 test-free released tree；本 fork 保留这一个聚焦测试以锁定时序与边界。若要与上游策略完全一致，可单独 revert `a4b641c`，不影响生产代码。

**发布构建与生产部署**

- 按上游 `release.yml` 的参数重建：`go build -trimpath -ldflags="-s -w -X main.version=v1.4.7+keepalive.1"`，Linux amd64 产物 14,110,880 字节，SHA256 `3B8FB6846047FA025A81CE3B796950394538CA83523B08C77FBAE26D257900FC`。
- VPS 冒烟使用 18080 临时实例与独立配置：`/healthz` 200、`/v1/models` 200、错误 Key 401；测试后端口释放，生产 8080 未受影响。
- 2026-11-10 15:59 CST 部署：备份原二进制为 `/opt/opencode2api/opencode2api.bak-20261110-155906`，原子替换后重启；健康检查 200，版本 `v1.4.7+keepalive.1`，WARP 2 个出口活跃，目录刷新后 `status=ok`。
- 生产鉴权复核：带 `server_keys` 请求 `/v1/models` 200，无 Key 401；公网 `https://oc.fxk.us.ci/healthz` 200。
- 已推送 `main`：功能提交与本文档记录均已同步到 `origin/main`。
- 自动更新风险：`.installed_version` 仍为 `v1.4.7`，`stack-update.timer` 在远端同版本时跳过；上游发布 `v1.4.8` 后会覆盖本自定义二进制。如需长期保留保活改动，应改为从本 fork 发布或调整更新源。
