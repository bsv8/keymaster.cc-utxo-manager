# keymaster.cc-utxo-manager

同一 UTXO manager 的原生 TypeScript / Go 实现（`utm/1`）。资金规则、请求/输出、端口、状态、错误和持久恢复必须对应；不是网页与 CLI 两套核心。

- TS：`typescript/src/index.ts`。
- Go：`go/utxo`，模块 `github.com/bsv8/keymaster.cc-utxo-manager`。
- 共同内核：`FundingKernel`；由宿主保证单写者和持久化原子提交。
- 宿主负责 HTTP、文件/进程锁、身份授权与签名器、网页/设备交互、CLI 参数和定时器。核心不启动后台查询或网络服务。

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

完整方法/字段/纯函数与缺省对照见工程库 [API 对照](../BSV8_Engineering/docs/keymaster.cc-utxo-manager/API对照.md)。公共视图（含协议 binding、外露责任和父输入数组）均是分离副本；修改视图不能改变资金状态。外露责任代表项按 ID 字典序选取，恢复前后稳定。outpoint 的 vout 必须是 uint32 规范十进制；损坏状态不能降级为空账。

端口为 `DurableStore`、`Broadcaster`、`ChainObserver`、可选 `InputObserver`；TS `OrchestrationPorts` 对应 Go `Ports`。TS可选 `saveState` 对应 Go `StateStore`；没有此能力时拒绝外露签名交付。金额使用十进制字符串；私钥不进入 manager。

`retryExact(kernel, ports, allocationId)` / `RetryExact(kernel, ports, allocationID)` 返回共同 `SubmitResult`：`allocationId`、`txid`、`feeSats`、`delivery`，有冲突结论时另带 `conclusion`。只读取既有钱包 outbox 与持久 exact；缺失/损坏、绑定不一致或外露交付责任拒绝；已完成申请返回稳定成功且不再调用广播端口。未知保留原输入责任，成功立即推进本钱包输出。提供 `InputObserver` 时先观察输入；可靠的本 tx 消费事实视为成功，其他消费方使原交易进入冲突收尾。冲突后不再重交 exact，而是继续逐输入核实；`failed` 返回稳定失败，不新选币。

`FundingKernel.resolveInputObservations` / `ResolveInputObservations` 接收同语义 `InputObservation`：outpoint、status（spent/unspent/unknown）；spent 还必须有 txid、vin、confirmed。manager不解释HTTP。仅确认仍未花且没有其它责任保护的输入恢复可选；未知/用途保护/外露部分签名/无效父依赖保持保护。outbox持久 `conclusion` 保存 releasedInputs、spentInputs、heldInputs、releasedSats；全部收尾后state=failed，allocation=aborted，原exact和去重墓碑继续保留。释放金额是该申请历史已恢复金额，不是当前余额；后续新申请占用同一输入不回归旧申请。

该 API 不启动循环。宿主必须通过显式入口授权每次查询会话，提供当前身份/权限控制及单写者；循环/退避/取消放宿主。重启只恢复记录，不能自动调用 `retryExact`。持久端口须将结果/完整状态一起原子提交，不能仅在内存中记住签名或回执。

工程测试、共享向量、双向恢复和故障证据位于同级 `BSV8_Engineering`，不放产品源码目录。新结论字段与failed状态需要支持该契约的阅读器；不要让旧实现忽略新字段后写回，或不备份直接降级。软件测试、发布签字、真实网络和实体设备测试是不同证据范围；接口说明不替代相应环境的验收。
