# keymaster.cc-utxo-manager

UTXO manager（`utm/1`）。本轮以 cc 内最新 TS 实现迁回本仓；cc 不再保留内置核心，消费本仓构建包。资金选币、占用和 exact/unknown 责任由唯一内核管理，宿主不在签名或广播前另设逐输入 spent 查询门禁；显式历史对账和未知输入结论仍通过观察端口处理。

- TS 唯一源码：`typescript/src/index.ts`；包名 `keymaster-utxo-manager`。
- Go 唯一源码：根目录 `utxo/`，`go.mod` 位于仓库根目录，module `github.com/bsv8/keymaster.cc-utxo-manager`；CLI 使用 `github.com/bsv8/keymaster.cc-utxo-manager/utxo`，不保留内部副本。Go 按最新 TS 语义同步，不能维护独立业务或格式规则。
- 共同内核：`FundingKernel`；由宿主保证单写者和持久化原子提交。
- 宿主负责 HTTP、文件/进程锁、身份授权与签名器、网页/设备交互、CLI 参数和定时器。核心不启动后台查询或网络服务。

## TS 构建与消费

在 `typescript/` 运行 `pnpm install && pnpm build`，生成 `dist/`；包只导出编译后的 JS 和声明文件，无运行时依赖。cc 使用已发布的精确 npm 版本 `keymaster-utxo-manager@0.1.0`，不通过源码 alias、本地 `file:` 依赖或内部副本接入。Go 目录整理不改变 npm 包。

工程仓 `node test/shared/component-packages.mjs` 验证打包后在临时目录独立安装和导入。组件测试和语料仍在工程仓 `test/shared/utxo-manager/`，不复制到产品目录。

## Go 构建与双语言回归

在仓库根目录运行 `go build ./...`，只依赖 Go 标准库，不需要 TS/Node。Go module 位于仓库根目录，使用根标签 `v0.1.1` 发布，不使用 `go/v...`；消费路径为 `github.com/bsv8/keymaster.cc-utxo-manager/utxo`。

工程仓统一入口 `npm run test:components:parity` 校验开发源码的 UTXO 逐步输出/错误码/状态哈希、字节一致快照与双向恢复续跑、随机轨迹与独立资金 oracle，以及调度器的完整状态/命令/视图/错误和跨语言恢复。具体命令见工程仓 `test/shared/README.md`。不同语言的内部调用同样受信，磁盘恢复同样检查损坏；不保留 Go 专属格式契约。TS 规则变化必须同步 Go 并补共享向量；有限测试通过不等于所有输入已被穷尽证明。

## 可选诊断（当前源码）

TS `setDebugSink(sink)` / `debugEnabled()`，Go `SetDebugSink(sink)` / `DebugEnabled()`；默认关闭，由宿主 dev 装配显式开启，不读取环境、不写文件、不依赖 BitFS。事件为 `{component, event, details}`，component=`utxo-manager`；时间戳/运行实例/序号由宿主日志出口补充，不进入业务状态。sink 必须快速返回、不重入业务；其异常被隔离。关闭时不构造状态摘要，开启时输出分离、脱敏的观察结果。

`operation.start/finish` 覆盖快照、保护来源、选币/替换/释放、签名前意图、协议与外露签名、派发、广播结果、未知/输入对账等，包含输入与前后状态，可以追踪 allocation、outpoint、金额/费用、保护与未决责任。`operation.rejected` 包含业务错误原因；Go 的拒绝事件由契约错误入口记录，当前操作由相邻边界事件关联，finish 本身不表示操作成功。`dispatch.persist.start/finish` 与 `broadcast.start/finish` 区分持久化和外部派发，不把内核候选变化冒充落盘。

日志禁止秘密、正文、原始交易、脚本、签名或预映像；二进制/原始材料只输出 byteLength，凭据字段隐藏。测试和长期结果保留在工程仓，日志不可用作资金事实或重放输入。

## 受信编排 API 对应

| TypeScript | Go | 用途 |
|---|---|---|
| `prepareIntent` | `PrepareIntent` | 签名前冻结并持久化普通交易意图 |
| `prepareProtocolIntent` | `PrepareProtocolIntent` | 冻结并持久化协议意图 |
| `completeProtocolIntent` | `CompleteProtocolIntent` | 保存协议 exact 与交付责任 |
| `recordExternalSignature` | `RecordExternalSignature` | 先持久再外露部分签名责任 |
| `submitSigned` | `SubmitSigned` | 验证普通签名交易并受控提交 |
| `submitProtocolSigned` | `SubmitProtocolSigned` | 验证协议签名交易并受控提交 |
| `reconcileUnknown` | `ReconcileUnknown` | 只读观察同一交易后持久结算，不广播 |
| `reconcileInputs` | `ReconcileInputs` | 只读逐 input 消费证据后持久结论/局部释放，不广播 |
| `retryExact` | `RetryExact` | 显式执行一次原 exact 重交，不选币/签名/变费 |

完整方法/字段/纯函数与缺省对照见工程库 [API 对照](../BSV8_Engineering/docs/shared/utxo-manager/API对照.md)。公共视图（含协议 binding、外露责任和父输入数组）均是分离副本；修改视图不能改变资金状态。外露责任代表项按 ID 字典序选取，恢复前后稳定。TS 内部调用者受信，不重复做调用参数格式认证；外部输入和磁盘恢复在边界检查，损坏状态不能降级为空账。

端口为 `DurableStore`、`Broadcaster`、`ChainObserver`、可选 `InputObserver`；TS `OrchestrationPorts` 对应 Go `Ports`。TS可选 `saveState` 对应 Go `StateStore`；没有此能力时拒绝外露签名交付。金额使用十进制字符串；私钥不进入 manager。

`retryExact(kernel, ports, allocationId)` / `RetryExact(kernel, ports, allocationID)` 返回共同 `SubmitResult`：`allocationId`、`txid`、`feeSats`、`delivery`，有冲突结论时另带 `conclusion`。只读取既有钱包 outbox 与持久 exact；缺失/损坏、绑定不一致或外露交付责任拒绝；已完成申请返回稳定成功且不再调用广播端口。未知保留原输入责任，成功立即推进本钱包输出。提供 `InputObserver` 时先观察输入；可靠的本 tx 消费事实视为成功，其他消费方使原交易进入冲突收尾。冲突后不再重交 exact，而是继续逐输入核实；`failed` 返回稳定失败，不新选币。

`FundingKernel.resolveInputObservations` / `ResolveInputObservations` 接收同语义 `InputObservation`：outpoint、status（spent/unspent/unknown）；spent 还必须有 txid、vin、confirmed。manager不解释HTTP。仅确认仍未花且没有其它责任保护的输入恢复可选；未知/用途保护/外露部分签名/无效父依赖保持保护。outbox持久 `conclusion` 保存 releasedInputs、spentInputs、heldInputs、releasedSats；全部收尾后state=failed，allocation=aborted，原exact和去重墓碑继续保留。释放金额是该申请历史已恢复金额，不是当前余额；后续新申请占用同一输入不回归旧申请。

该 API 不启动循环。宿主必须通过显式入口授权每次查询会话，提供当前身份/权限控制及单写者；循环/退避/取消放宿主。重启只恢复记录，不能自动调用 `retryExact`。持久端口须将结果/完整状态一起原子提交，不能仅在内存中记住签名或回执。

工程测试、共享向量、双向恢复和故障证据位于同级 `BSV8_Engineering`，不放产品源码目录。新结论字段与failed状态需要支持该契约的阅读器；不要让旧实现忽略新字段后写回，或不备份直接降级。软件测试、发布签字、真实网络和实体设备测试是不同证据范围；接口说明不替代相应环境的验收。
