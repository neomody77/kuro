# Kuro 测试计划

> 本文档描述各模块的测试需求，细化为具体用例。
> 测试分三层：单元测试（`_test.go`）、集成测试（跨模块）、端到端测试（Spier 浏览器自动化）。

## 原则

- 每个模块可独立测试，不依赖整体服务启动
- 依赖外部模块的地方通过 interface mock 隔离
- 核心 interface：`ActionHandler`、`ExecutionStore`、`provider.Provider`
- 端到端测试使用 `/spier` 通过浏览器 runtime 进行 UI + API 全链路验证

---

## 1. Credential（凭据）

### 1.1 加密核心 (`credential.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 1.1.1 | 生成 master key | 临时路径 | 文件 32 字节，权限 0600 |
| 1.1.2 | 加载 master key | 已生成的 key 文件 | 与生成值一致 |
| 1.1.3 | 加载无效长度 key | 16 字节文件 | 返回 error |
| 1.1.4 | 加载不存在文件 | 无效路径 | 返回 error |
| 1.1.5 | 加密后密文不等于明文 | "secret" | `ENC[AES256:...]` 格式，值不同 |
| 1.1.6 | 加密-解密往返 | "hello world" | 解密后恢复原文 |
| 1.1.7 | 不同 key 无法解密 | key A 加密，key B 解密 | 返回 error |
| 1.1.8 | 解密无效格式 | "not-encrypted" | 返回 error |
| 1.1.9 | 加密使用不同 nonce | 同一明文连续加密两次 | 两次密文不同 |
| 1.1.10 | IsEncrypted 判断 | 有效/无效/缺括号格式 | true/false/false |

### 1.2 凭据校验 (`credential.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 1.2.1 | 空 name 拒绝 | `Name: ""` | error |
| 1.2.2 | 未知 type 拒绝 | `Type: "unknown"` | error |
| 1.2.3 | email 类型缺字段 | 缺少 imap_host | error |
| 1.2.4 | generic 类型接受任意字段 | 空 Data | 通过 |
| 1.2.5 | 所有内置类型必填字段验证 | email/http-basic/http-bearer/openai/anthropic/telegram-bot/generic | 各自通过或失败 |
| 1.2.6 | YAML 序列化-反序列化往返 | 完整 Credential | 往返后字段一致 |

### 1.3 Store CRUD (`store.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 1.3.1 | Save 后文件内容已加密 | 明文 Credential | 文件中值以 `ENC[AES256:` 开头 |
| 1.3.2 | Save 产生 git commit | 保存凭据 | gitstore.Log 有新提交 |
| 1.3.3 | Save + Get 往返 | 保存后读取 | 解密后 Data 与原始一致 |
| 1.3.4 | List 不返回 Data | 保存多个凭据后 List | 仅返回 Name/Type，无 Data |
| 1.3.5 | List 空目录 | 空 store | 返回空切片，无 error |
| 1.3.6 | Delete 文件消失 | 删除已存在凭据 | 文件不存在 + git commit |
| 1.3.7 | Delete 不存在凭据 | 删除未知 name | 返回 error |
| 1.3.8 | Get 不存在凭据 | 读取未知 name | 返回 error |
| 1.3.9 | 更新已有凭据 | Save 同名凭据两次 | Get 返回最新值 |
| 1.3.10 | 已加密值不二次加密 | Data 中包含 `ENC[AES256:...]` | 保持原密文 |
| 1.3.11 | 多凭据共存 | 保存 3 个不同凭据 | List 返回 3 条 |

### 1.4 隔离性

| # | 用例 | 期望结果 |
|---|------|----------|
| 1.4.1 | pipeline 调用 credential skill 返回解密值 | Get 操作返回明文 |
| 1.4.2 | chat 通过 skill 调用 list 不暴露密文 | List 无 Data 字段 |

---

## 2. Document（文档）

### 2.1 CRUD

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 2.1.1 | Put 写入文件并 commit | path + content | 文件存在 + git commit |
| 2.1.2 | Put 自动创建嵌套目录 | `a/b/c/doc.md` | 目录链自动创建 |
| 2.1.3 | Put 覆盖已有文件 | 同路径写两次 | Get 返回最新内容 |
| 2.1.4 | Get 读取内容 | 已存在文件 | 返回 Doc，含 Content |
| 2.1.5 | Get 嵌套路径 | `notes/2026/march.md` | 正确读取 |
| 2.1.6 | Get 不存在文件 | 无效路径 | 返回 error |
| 2.1.7 | Get 目录路径 | 传入目录路径 | 返回 error（目录不可 Get） |
| 2.1.8 | Delete 删除文件并 commit | 已存在文件 | 文件消失 + git commit |
| 2.1.9 | Delete 不存在文件 | 无效路径 | 无 error |

### 2.2 List

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 2.2.1 | 列出嵌套文件夹 | 含子目录结构 | 返回 Doc 列表含 IsDir 标记 |
| 2.2.2 | 列出空目录 | 空 store | 返回空切片 |
| 2.2.3 | 列出不存在目录 | 无效路径 | 返回 error |

### 2.3 搜索

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 2.3.1 | 关键词命中 | 包含关键词的文件 | 返回匹配 Doc |
| 2.3.2 | 无结果 | 不存在关键词 | 返回空切片 |
| 2.3.3 | 嵌套文件搜索 | 子目录中的文件 | 也能被搜到 |

### 2.4 安全

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 2.4.1 | 路径穿越 `../` 拒绝 | `../../etc/passwd` | error |
| 2.4.2 | 路径穿越 `foo/../../` 拒绝 | `foo/../../secret` | error |
| 2.4.3 | 合法嵌套路径通过 | `notes/sub/file.md` | 通过 |

### 2.5 Git 集成

| # | 用例 | 期望结果 |
|---|------|----------|
| 2.5.1 | Put 无 git 也能工作 | git=nil 时文件正常写入 |
| 2.5.2 | Put 有 git 自动 commit | gitstore.Log 有对应提交 |

---

## 3. Chat（对话）

### 3.1 会话生命周期

| # | 用例 | 期望结果 |
|---|------|----------|
| 3.1.1 | 创建会话 | 返回 SessionInfo（ID + Title + Created） |
| 3.1.2 | 列出会话 | 返回用户所有 SessionInfo |
| 3.1.3 | 删除会话 | 会话从 List 中消失 |
| 3.1.4 | 获取历史 | 返回该会话 Message 列表 |
| 3.1.5 | 新用户空历史 | 返回空切片 |
| 3.1.6 | 空 sessionID 默认为 "default" | 自动创建 default 会话 |
| 3.1.7 | 首条消息自动设 title | 截取前 40 字符 |

### 3.2 用户隔离

| # | 用例 | 期望结果 |
|---|------|----------|
| 3.2.1 | 用户 A 看不到用户 B 的会话 | ListSessions 隔离 |
| 3.2.2 | 用户 A 看不到用户 B 的历史 | GetHistory 隔离 |

### 3.3 AI 响应解析

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 3.3.1 | 纯文本响应 | 无 skill 块 | Response.SkillCall == nil |
| 3.3.2 | 含 skill 块 | ` ```skill {"skill":"http",...}``` ` | 正确解析 SkillCall |
| 3.3.3 | 无效 skill JSON | 格式错误的 JSON | 作为纯文本处理 |
| 3.3.4 | 系统 prompt 包含 skill 列表 | 注册了 skills | prompt 含 `{{SKILLS}}` 展开内容 |

### 3.4 Skill 调用流程

| # | 用例 | 期望结果 |
|---|------|----------|
| 3.4.1 | 非破坏性 skill 立即执行 | 结果嵌入 Message.Content |
| 3.4.2 | 破坏性 skill 等待确认 | Response.SkillCall.Confirm == true |
| 3.4.3 | 确认执行 | ConfirmAction(approve=true) 执行并返回结果 |
| 3.4.4 | 拒绝执行 | ConfirmAction(approve=false) 返回取消消息 |
| 3.4.5 | 无 pending 时确认 | 返回 error |

### 3.5 破坏性 skill 判定

| # | 用例 | 期望结果 |
|---|------|----------|
| 3.5.1 | shell | 破坏性 |
| 3.5.2 | credential:delete | 破坏性 |
| 3.5.3 | document:delete | 破坏性 |
| 3.5.4 | file:write | 破坏性 |
| 3.5.5 | http | 非破坏性 |
| 3.5.6 | document:list | 非破坏性 |

### 3.6 Provider 切换

| # | 用例 | 期望结果 |
|---|------|----------|
| 3.6.1 | SetProvider 运行时切换 | 后续请求使用新 provider |
| 3.6.2 | 多轮对话上下文保持 | 切换后历史不丢失 |

---

## 4. Skill（技能）

### 4.1 Registry

| # | 用例 | 期望结果 |
|---|------|----------|
| 4.1.1 | Register + Get | 注册后能按 name 找到 |
| 4.1.2 | List | 返回所有已注册 skill |
| 4.1.3 | Delete | 删除后 Get 返回 nil |
| 4.1.4 | RegisterDefaults | 注册 10 个核心 skill |
| 4.1.5 | 所有核心 skill 实现 ActionHandler | Handler != nil |
| 4.1.6 | Execute 未知 skill | 返回 error |
| 4.1.7 | Execute 缺必填参数 | 返回 error |
| 4.1.8 | Skill JSON 序列化 | 往返一致 |

### 4.2 credential skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.2.1 | list | `action: list` | 返回凭据列表，无 Data |
| 4.2.2 | get | `action: get, name: X` | 返回解密后完整凭据 |
| 4.2.3 | save | `action: save, name/type/data` | 保存成功 |
| 4.2.4 | save (map[string]any 类型 data) | data 为 `map[string]any` | 自动转换为 `map[string]string` |
| 4.2.5 | delete | `action: delete, name: X` | 删除成功 |
| 4.2.6 | get 缺 name | 无 name 参数 | 返回 error |

### 4.3 document skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.3.1 | list | `action: list` | 返回文档列表 |
| 4.3.2 | get | `action: get, path: X` | 返回文档内容 |
| 4.3.3 | save | `action: save, path/content` | 保存成功 |
| 4.3.4 | delete | `action: delete, path: X` | 删除成功 |
| 4.3.5 | search | `action: search, query: X` | 返回匹配文档 |
| 4.3.6 | get 缺 path | 无 path 参数 | 返回 error |

### 4.4 shell skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.4.1 | 执行命令 | `echo hello` | stdout: "hello\n" |
| 4.4.2 | 捕获 stderr | `echo err >&2` | stderr 在结果中 |
| 4.4.3 | exit code 非零 | `exit 1` | error 包含退出码 |

### 4.5 file skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.5.1 | read + write 往返 | 写入后读取 | 内容一致 |
| 4.5.2 | list 目录 | 含文件的目录 | 返回文件列表 |

### 4.6 transform skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.6.1 | jq 过滤 | `.name` | 提取字段 |
| 4.6.2 | jq map | `[.[] \| .id]` | 映射数组 |
| 4.6.3 | jq select | `[.[] \| select(.x > 1)]` | 过滤数组 |
| 4.6.4 | jq length | `. \| length` | 返回长度 |

### 4.7 template skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.7.1 | Go 模板渲染 | `Hello {{.Name}}` + data | "Hello World" |

### 4.8 http skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.8.1 | GET 请求 | httptest server URL | 返回状态码 + body |

### 4.9 ai skill

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 4.9.1 | completion 调用 | mock provider + prompt | 返回 mock 响应 |

---

## 5. Pipeline

### 5.1 DAG (`dag.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 5.1.1 | 线性链拓扑排序 | A→B→C | [A, B, C] |
| 5.1.2 | 环检测 | A→B→A | DetectCycles() == true |
| 5.1.3 | 无环检测 | A→B→C | DetectCycles() == false |
| 5.1.4 | 根节点查找 | A→B, C→D | roots = [A, C] |
| 5.1.5 | 菱形并行组 | A→{B,C}→D | groups = [[A],[B,C],[D]] |
| 5.1.6 | 含环并行组 | A→B→A | 返回 error |

### 5.2 Executor (`executor.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 5.2.1 | 线性执行 | A→B→C，各 mock handler | 按序执行，所有 status=success |
| 5.2.2 | 并行执行 | A→{B,C}→D | B/C 实际并行（验证时间） |
| 5.2.3 | 节点失败 | B 返回 error | Execution.Status=error |
| 5.2.4 | 未知 action | action 未注册 | error |
| 5.2.5 | Context 取消 | 执行中取消 ctx | 停止执行 |
| 5.2.6 | 执行结果存储 | 提供 ExecutionStore | Execution 被 Save |
| 5.2.7 | 执行记录包含时间 | 任意执行 | 每个节点有 StartTime + ExecutionTime |

### 5.3 Parser (`parser.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 5.3.1 | 合法 JSON 解析 | 完整 workflow JSON | 返回 Workflow 对象 |
| 5.3.2 | 空输入拒绝 | `[]byte{}` | error |
| 5.3.3 | 非 JSON 拒绝 | `"not json"` | error |
| 5.3.4 | 截断 JSON 拒绝 | `{"name":` | error |
| 5.3.5 | 错误类型拒绝 | `[1,2,3]` | error |
| 5.3.6 | 校验通过 | 有 name + nodes + 合法 connections | 无 error |
| 5.3.7 | 缺 name 校验失败 | `Name: ""` | error |
| 5.3.8 | 无 nodes 校验失败 | `Nodes: []` | error |
| 5.3.9 | connection 引用不存在节点 | 引用未定义 node | error |
| 5.3.10 | 无 ID 自动生成 | `ID: ""` | 解析后 ID 非空 |

### 5.4 Expression (`expr.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 5.4.1 | 节点 output 引用 | `{{ nodes.fetch.output }}` | 返回节点输出值 |
| 5.4.2 | 节点 status 引用 | `{{ nodes.fetch.status }}` | "success"/"error" |
| 5.4.3 | 节点 error 引用 | `{{ nodes.fetch.error }}` | 错误消息 |
| 5.4.4 | 节点 duration 引用 | `{{ nodes.fetch.duration }}` | 执行时长 |
| 5.4.5 | Map 字段引用 | `{{ nodes.fetch.someField }}` | map 中的值 |
| 5.4.6 | 不存在的节点 | `{{ nodes.unknown.output }}` | 空字符串 |
| 5.4.7 | `now` 关键词 | `{{ now }}` | RFC3339 时间戳 |
| 5.4.8 | `date` 过滤器 | `{{ now \| date('YYYY-MM-DD') }}` | 格式化日期 |
| 5.4.9 | `length` 过滤器 | `{{ nodes.x.output \| length }}` | 字符串长度 |
| 5.4.10 | `contains` 过滤器 | `{{ nodes.x.output \| contains('foo') }}` | "true"/"false" |
| 5.4.11 | `upper` 过滤器 | `{{ nodes.x.output \| upper }}` | 大写字符串 |
| 5.4.12 | `lower` 过滤器 | `{{ nodes.x.output \| lower }}` | 小写字符串 |
| 5.4.13 | 多表达式混合 | `Hello {{ a }} and {{ b }}` | 所有占位符替换 |
| 5.4.14 | 无表达式 | 纯文本 | 原样返回 |
| 5.4.15 | ResolveParams | params map 中含表达式 | 所有值被解析 |

### 5.5 Scheduler (`scheduler.go`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 5.5.1 | 每分钟 cron | `* * * * *` | 下次执行 = 下一分钟 |
| 5.5.2 | 指定时间 | `30 14 * * *` | 14:30 |
| 5.5.3 | 尚未到达的时间 | `0 10 * * *`（当前 9:00） | 当天 10:00 |
| 5.5.4 | 指定星期几 | `0 0 * * 1` | 下一个周一 |
| 5.5.5 | 非法表达式 | `"invalid"` | 降级处理（默认 1 小时） |
| 5.5.6 | 通配符解析 | `*` | 所有值 |
| 5.5.7 | 单值解析 | `5` | [5] |
| 5.5.8 | 范围解析 | `1-3` | [1, 2, 3] |
| 5.5.9 | 逗号分隔 | `1,3,5` | [1, 3, 5] |
| 5.5.10 | 超出范围 | `60`（分钟字段） | error |
| 5.5.11 | matchesCron | 时间与表达式匹配 | true/false |

### 5.6 Store (`store.go`)

#### 5.6.1 WorkflowStore

| # | 用例 | 期望结果 |
|---|------|----------|
| 5.6.1.1 | Save + Get 往返 | 字段一致 |
| 5.6.1.2 | List | 返回所有 workflow |
| 5.6.1.3 | Delete | 文件消失 |
| 5.6.1.4 | List 空目录 | 返回空切片 |
| 5.6.1.5 | Delete 不存在 | 无 error |
| 5.6.1.6 | Save 自动创建目录 | 目录不存在时自动创建 |
| 5.6.1.7 | Save 设置时间戳 | CreatedAt/UpdatedAt 非零 |

#### 5.6.2 ExecutionStore

| # | 用例 | 期望结果 |
|---|------|----------|
| 5.6.2.1 | Save + Get 往返 | 字段一致 |
| 5.6.2.2 | ListExecutions 按时间倒序 | 最新的在前 |
| 5.6.2.3 | ListExecutions limit 限制 | 返回数量 <= limit |
| 5.6.2.4 | ListExecutions 按 workflowID 过滤 | 只返回指定 workflow 的执行 |
| 5.6.2.5 | Get 不存在 | 返回 error |
| 5.6.2.6 | List 空 store | 返回空切片 |

#### 5.6.3 VariableStore / TagStore / DataTableStore

| # | 用例 | 期望结果 |
|---|------|----------|
| 5.6.3.1 | Variable CRUD | Create → Get → Update → Delete 往返 |
| 5.6.3.2 | Tag CRUD | Create → Get → Update → Delete 往返 |
| 5.6.3.3 | DataTable CRUD | CreateTable → GetTable → UpdateTable → DeleteTable |
| 5.6.3.4 | DataTable 行操作 | InsertRows → ListRows → UpdateRow → DeleteRow |
| 5.6.3.5 | 自动生成 ID | Create 时 ID 为空 → 自动分配 |

---

## 6. API

### 6.1 通用

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.1.1 | 所有端点 JSON 响应 | Content-Type: application/json |
| 6.1.2 | 未知路由 | 404 |
| 6.1.3 | 错误码格式一致 | `{"error": "message"}` |

### 6.2 认证中间件

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 6.2.1 | 有效 token | `Authorization: Bearer tok_abc` | 200，识别用户 |
| 6.2.2 | 无效 token | `Bearer invalid` | 401 |
| 6.2.3 | 无 token（单用户模式） | 无 header | 200（默认用户） |
| 6.2.4 | query param token | `?token=tok_abc` | 200，识别用户 |

### 6.3 用户数据隔离

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.3.1 | 用户 A 的 pipeline 列表不含 B 的 | 各自 repo 隔离 |
| 6.3.2 | 用户 A 的 credential 列表不含 B 的 | 各自加密隔离 |
| 6.3.3 | 用户 A 的 document 列表不含 B 的 | 各自目录隔离 |

### 6.4 端点组

#### 6.4.1 Health

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.1.1 | GET /api/health | 200 + `{"status":"ok"}` |
| 6.4.1.2 | 无需认证 | 无 token 也返回 200 |

#### 6.4.2 Workflows (n8n 兼容)

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.2.1 | GET /api/v1/workflows | 返回 workflow 列表 |
| 6.4.2.2 | POST /api/v1/workflows | 创建 workflow，返回 201 |
| 6.4.2.3 | GET /api/v1/workflows/{id} | 返回指定 workflow |
| 6.4.2.4 | PUT /api/v1/workflows/{id} | 更新 workflow |
| 6.4.2.5 | DELETE /api/v1/workflows/{id} | 删除 workflow |
| 6.4.2.6 | POST /api/v1/workflows/{id}/activate | 设置 Active=true |
| 6.4.2.7 | POST /api/v1/workflows/{id}/deactivate | 设置 Active=false |
| 6.4.2.8 | GET /api/v1/workflows/{id} 不存在 | 404 |

#### 6.4.3 Executions

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.3.1 | GET /api/v1/executions | 返回执行记录列表 |
| 6.4.3.2 | GET /api/v1/executions/{id} | 返回指定执行记录 |
| 6.4.3.3 | DELETE /api/v1/executions/{id} | 删除执行记录 |

#### 6.4.4 Credentials

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.4.1 | GET /api/v1/credentials | 列表（无密文） |
| 6.4.4.2 | POST /api/v1/credentials | 创建凭据 |
| 6.4.4.3 | GET /api/v1/credentials/{id} | 返回凭据详情 |
| 6.4.4.4 | PATCH /api/v1/credentials/{id} | 部分更新凭据 |
| 6.4.4.5 | DELETE /api/v1/credentials/{id} | 删除凭据 |

#### 6.4.5 Variables / Tags / DataTables

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.5.1 | Variable CRUD | 4 个端点正常工作 |
| 6.4.5.2 | Tag CRUD | 4 个端点正常工作 |
| 6.4.5.3 | DataTable CRUD + 行操作 | 表和行的 CRUD 正常 |

#### 6.4.6 Documents

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.6.1 | GET /api/documents | 列出文档 |
| 6.4.6.2 | GET /api/documents/*path | 获取文档内容 |
| 6.4.6.3 | PUT /api/documents/*path | 创建/更新文档 |
| 6.4.6.4 | DELETE /api/documents/*path | 删除文档 |
| 6.4.6.5 | 路径穿越拒绝 | `../etc/passwd` → 400 |

#### 6.4.7 Chat

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.7.1 | POST /api/chat/sessions 创建会话 | 201 |
| 6.4.7.2 | GET /api/chat/sessions 列表 | 返回当前用户会话 |
| 6.4.7.3 | DELETE /api/chat/sessions?id=X | 删除会话 |
| 6.4.7.4 | POST /api/chat 发送消息 | 返回 AI 响应 |
| 6.4.7.5 | GET /api/chat/history?session=X | 返回历史 |
| 6.4.7.6 | POST /api/chat/confirm | 确认/拒绝破坏性操作 |

#### 6.4.8 Settings

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.8.1 | GET /api/settings | 返回当前配置 |
| 6.4.8.2 | PUT /api/settings/model | 切换 active model |
| 6.4.8.3 | GET /api/settings/providers | 列出 provider |
| 6.4.8.4 | POST /api/settings/providers | 添加 provider |
| 6.4.8.5 | DELETE /api/settings/providers/{id} | 删除 provider |
| 6.4.8.6 | POST /api/settings/providers/{id}/test | 测试连接 |

#### 6.4.9 Skills

| # | 用例 | 期望结果 |
|---|------|----------|
| 6.4.9.1 | GET /api/skills | 列出所有 skill |
| 6.4.9.2 | GET /api/skills/{id} | 返回 skill 详情 |

---

## 7. GitStore

| # | 用例 | 期望结果 |
|---|------|----------|
| 7.1 | Init 创建 .git 目录 | `.git` 存在 |
| 7.2 | Init 幂等 | 已有 repo 再次 Init 不报错 |
| 7.3 | Open 已有 repo | 返回 Store |
| 7.4 | Open 非 git 目录 | 返回 error |
| 7.5 | Add + Commit 出现在 Log 中 | Log 返回对应 Commit |
| 7.6 | Log 返回 Hash + Message + Time | 三字段非空 |
| 7.7 | Log 限制条数 | `Log(2)` 最多返回 2 条 |
| 7.8 | Log 空仓库 | 返回 nil，无 error |
| 7.9 | Revert 撤销提交 | 文件内容恢复 |
| 7.10 | Diff 返回补丁 | 包含新增行的 diff |
| 7.11 | Status 检测修改 | 已修改文件显示 M |
| 7.12 | Status 检测未跟踪 | 新文件显示 ?? |
| 7.13 | Status 干净仓库 | 返回空字符串 |

---

## 8. Settings / Provider

### 8.1 Settings (`settings.go`)

| # | 用例 | 期望结果 |
|---|------|----------|
| 8.1.1 | 加载默认设置 | 无配置文件时使用默认值 |
| 8.1.2 | 保存 + 加载往返 | YAML 持久化后读回一致 |
| 8.1.3 | Active model 设置和读取 | 设置后可读取 |

### 8.2 Provider (`provider.go`, `openai.go`)

| # | 用例 | 期望结果 |
|---|------|----------|
| 8.2.1 | Provider 注册 | 添加后可查找 |
| 8.2.2 | Provider 删除 | 删除后不可查找 |
| 8.2.3 | Provider 列表 | 返回所有注册 provider |
| 8.2.4 | Provider 连接测试 | mock server 返回成功/失败 |
| 8.2.5 | OpenAI provider Complete 调用 | mock 返回 completion |
| 8.2.6 | 运行时切换 provider | 新请求走新 provider |

---

## 9. Recovery

### 9.1 Health (`recovery/health/`)

| # | 用例 | 期望结果 |
|---|------|----------|
| 9.1.1 | 创建 Checker | 默认 autoRollback=true |
| 9.1.2 | 初始 failure 计数 | 任意 version = 0 |
| 9.1.3 | 初始 lastKnownGood | 空字符串 |
| 9.1.4 | ping 健康服务 | 返回 nil |
| 9.1.5 | ping 不健康服务 | 返回 error |
| 9.1.6 | ping 无响应服务 | 返回 error |
| 9.1.7 | 健康检查重置 failure 计数 | 连续成功后 failures=0 |
| 9.1.8 | failure 累加 | 连续失败 n 次 = n |
| 9.1.9 | 达到阈值触发 rollback | failures >= threshold 且有 LKG → 启动 LKG |
| 9.1.10 | 无 LKG 不 rollback | lastKnownGood="" → 跳过 |
| 9.1.11 | LKG 等于 failed 不 rollback | 同版本 → 跳过 |
| 9.1.12 | 跳过 stopped 版本 | 不 ping 已停止版本 |
| 9.1.13 | 健康的 default 设为 LKG | default 健康 → lastKnownGood 更新 |
| 9.1.14 | SetAutoRollback 切换 | true ↔ false |
| 9.1.15 | Start / Stop 生命周期 | 启动后可停止，不 panic |

### 9.2 Proxy (`recovery/proxy/`)

| # | 用例 | 输入 | 期望结果 |
|---|------|------|----------|
| 9.2.1 | 版本未运行 | `?v=0.1.0` 但未启动 | 503 |
| 9.2.2 | 无默认版本 | 无 `?v=`，无 current | 503 |
| 9.2.3 | 不存在版本 | `?v=9.9.9` | 503 |
| 9.2.4 | 默认版本未运行 | current 存在但 stopped | 503 |
| 9.2.5 | `?v=` 参数剥离 | 转发请求不含 `v` 参数 | 后端收到无 `v` 的请求 |
| 9.2.6 | 端到端转发 | mock backend | 响应正确透传 |

### 9.3 Version (`recovery/version/`)

| # | 用例 | 期望结果 |
|---|------|----------|
| 9.3.1 | Scan 发现版本目录 | 列出所有子目录作为版本 |
| 9.3.2 | Scan 不存在目录 | 自动创建 baseDir |
| 9.3.3 | Scan 幂等 | 多次 Scan 不重复 |
| 9.3.4 | Get 指定版本 | 返回 Version 对象 |
| 9.3.5 | SetDefault 创建 symlink | `current` 指向目标版本 |
| 9.3.6 | Install 二进制 | 文件复制到版本目录 |
| 9.3.7 | Install 含 UI | binary + ui/ 都复制 |
| 9.3.8 | Delete 版本 | 目录移除 |
| 9.3.9 | Delete 当前版本被拒绝 | 返回 error |
| 9.3.10 | Delete 不存在版本 | 返回 error |
| 9.3.11 | Start 不存在版本 | 返回 error |
| 9.3.12 | Start 无二进制 | 返回 error |
| 9.3.13 | Stop 未运行版本 | 返回 error |
| 9.3.14 | Stop 不存在版本 | 返回 error |
| 9.3.15 | PortForVersion | running 返回端口，stopped 返回 0 |
| 9.3.16 | DefaultPort 无默认 | 返回 0 |
| 9.3.17 | 端口分配递增 | 连续启动使用不同端口 |
| 9.3.18 | List 排序 | 版本号降序排列 |

---

## 10. 端到端测试（Spier 浏览器自动化）

> 使用 `/spier` 通过 Chrome DevTools Protocol 进行浏览器级别的全链路验证。
> 需要先启动 kuro 服务 (`./kuro`)。

### 10.1 UI 导航

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.1.1 | 页面加载 | 打开 `http://localhost:8080` | 页面正常渲染，无 console error |
| 10.1.2 | 侧边栏导航 | 依次点击 Chat/Pipelines/Skills/Documents/Vault/Logs/Settings | 各页面正确切换 |
| 10.1.3 | 移动端布局 | 设置 viewport 宽度 375px | 底部 tab 导航出现 |

### 10.2 Chat 全链路

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.2.1 | 发送消息 | 在 Chat 页输入文本并发送 | AI 响应出现在对话中 |
| 10.2.2 | 多轮对话 | 连续发送 3 条消息 | 历史记录完整 |
| 10.2.3 | 新建会话 | 点击新建会话 | 清空对话区 |
| 10.2.4 | 切换会话 | 选择历史会话 | 加载该会话消息 |

### 10.3 Pipeline 全链路

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.3.1 | 创建 pipeline | 在 Pipelines 页创建新 workflow | 列表中出现新条目 |
| 10.3.2 | 查看详情 | 点击 pipeline 条目 | 显示 DAG 图 + 配置 |
| 10.3.3 | 手动运行 | 点击 "Run Now" | 执行完成，显示结果 |
| 10.3.4 | 查看运行历史 | 运行后查看 Runs tab | 显示执行记录 |
| 10.3.5 | 删除 pipeline | 点击删除 | 从列表消失 |

### 10.4 Vault 全链路

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.4.1 | 创建凭据 | 在 Vault 页填写表单并保存 | 列表中出现新凭据 |
| 10.4.2 | 查看凭据 | 点击已有凭据 | 显示类型和字段名，不显示密文 |
| 10.4.3 | 删除凭据 | 点击删除 | 从列表消失 |

### 10.5 Documents 全链路

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.5.1 | 创建文档 | 新建 Markdown 文档 | 文件出现在列表 |
| 10.5.2 | 编辑文档 | 修改内容并保存 | 重新打开后内容已更新 |
| 10.5.3 | 目录浏览 | 点击子目录 | 显示子目录内容 |
| 10.5.4 | 删除文档 | 点击删除 | 从列表消失 |

### 10.6 Settings 全链路

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.6.1 | 添加 provider | 在 Settings 页添加 OpenAI provider | 列表中出现 |
| 10.6.2 | 切换 active model | 选择不同 model | 设置保存成功 |
| 10.6.3 | 删除 provider | 点击删除 | 从列表消失 |

### 10.7 网络请求验证

| # | 用例 | 操作 | 期望结果 |
|---|------|------|----------|
| 10.7.1 | API 请求格式 | 通过 Spier 抓取 network 请求 | 所有 API 调用返回 200/201 |
| 10.7.2 | 无 console error | 操作各页面 | console 无 error/warning |
| 10.7.3 | 认证 token 传递 | 检查请求 header | 包含 Authorization header |

---

## 11. 审计日志（待实现后补充）

| # | 用例 | 期望结果 |
|---|------|----------|
| 11.1 | 日志写入完整性 | 每个操作都有记录 |
| 11.2 | trace ID 贯穿调用链 | 同一请求的日志有相同 trace ID |
| 11.3 | 按时间/类型/会话筛选 | 过滤条件生效 |
| 11.4 | 异常持久化 | 崩溃前的日志不丢失 |

---

## 附录 A：Spier 端到端测试发现的问题（2026-03-08）

### A.1 已修复

| # | 问题 | 严重程度 | 位置 | 状态 |
|---|------|----------|------|------|
| A.1.1 | **SPA 路由刷新 404** — 直接访问 `/pipelines`、`/skills` 等客户端路由时返回 `404 page not found`。原因：`server.ServeUI` 使用 `http.FileServer` 不支持 SPA fallback，嵌入 FS 中无对应文件时直接 404。 | 高 | `internal/server/server.go:ServeUI` | 已修复 — 添加 SPA fallback 逻辑，非静态资源路径回退到 `index.html` |

### A.2 待修复

| # | 问题 | 严重程度 | 位置 | 说明 |
|---|------|----------|------|------|
| A.2.1 | **Logs 页面 "Invalid Date"** — 执行记录的 Time 列全部显示 "Invalid Date"，Pipeline 列为空。 | 中 | `ui/src/pages/Logs.tsx` + `internal/pipeline/store.go` | 前端日期解析与后端返回的时间格式不匹配，需检查 `startedAt` 字段格式 |
| A.2.2 | **Logs 页面 "0 nodes"** — 所有执行记录的 Nodes 列显示 "0 nodes"。 | 低 | `ui/src/pages/Logs.tsx` | 前端可能没有正确读取 `data.resultData.runData` 中的节点数 |
| A.2.3 | **Pipeline Run 按钮无 node handler** — `http`、`transform` 等节点类型未注册 handler，手动运行 pipeline 总是报 "unknown node type" 错误。 | 中 | `cmd/kuro/main.go` executor handler 注册 | 目前仅注册了 IMAP/If/SMTP/Cron 四种 handler，缺少 http/transform/file/shell 等通用 handler |

### A.3 测试通过项

| # | 用例 | 结果 |
|---|------|------|
| 1 | 首页加载（Chat 页） | 正常渲染，侧边栏完整 |
| 2 | 7 个页面导航（Chat/Pipelines/Skills/Documents/Vault/Logs/Settings） | 全部正确切换 |
| 3 | Pipelines 列表 | 6 个 workflow 正确显示名称和节点数 |
| 4 | Pipeline 详情 | 节点列表、时区、创建时间正确 |
| 5 | Skills 列表 | 10 个核心 skill 卡片正确显示 |
| 6 | Documents 文件树 + 内容预览 | 嵌套目录 `note/test-doc.md` 正常，Markdown 渲染正确 |
| 7 | Vault 凭据列表 | 3 个凭据正确显示，不暴露密文 |
| 8 | Settings Provider 配置 | OpenRouter 配置正确显示 |
| 9 | API 请求全部 200 | `/api/pipelines`、`/api/skills`、`/api/documents`、`/api/credentials`、`/api/logs`、`/api/settings` |
| 10 | 全程零 JS 错误 | console error = 0，runtime error = 0 |
