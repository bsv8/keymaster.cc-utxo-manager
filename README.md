# keymaster.cc-utxo-manager

受信内部使用的内存 UTXO 分配池。两个进项：WoC 可用 UTXO、宿主交付的当前钱包可用 P2PKH UTXO；分配即冻结，只有调用者主动释放才回池。成功找零立即可分配，不等待 WoC。

- TypeScript：`typescript/src/`，npm 包 `keymaster-utxo-manager`；构建：`cd typescript && npm install && npm run build`。
- Go：根 module `github.com/bsv8/keymaster.cc-utxo-manager`，导入 `.../utxo`；构建：`go build ./...`。
- 无运行时依赖。调用者负责身份、并发串行、构建、签名、广播和结果判断。核心不做许可、签名验证、mempool 追踪、冲突取证或持久化恢复。

## API

`FundingKernel` / `NewFundingKernel`：

- `applySnapshot` / `ApplySnapshot`：合并完整 WoC 可用集合。
- `request` / `Request`：按金额/费用选币并立即冻结。
- `requestAll` / `RequestAll`：仅接收 requestId，同步冻结全部当前可分配钱包输入，返回 allocationId、完整输入材料和 totalInSats；不接收费率或收款脚本，不计算交易费用及输出。
- `assertAllocationUsable` / `AssertAllocationUsable`：仅检查 held 分配，无返回数据；申请成功一次交付构建所需的输入材料。
- `release` / `Release`：调用者显式释放分配，不核查错误证明或签名状态。
- `injectP2pkhUtxos` / `InjectP2pkhUtxos`：可用 P2PKH UTXO 注入，接收 `[{txid,vout,value}]`，立即可分配、不等 WoC 或区块确认。宿主保证归属、金额和可花费性；manager 不检查脚本类型或归属，统一使用实例脚本。同一交易可分批注入，按 outpoint 去重，不保存成功交易记录；无自有输出不调用。
- `funds` / `Funds`：只返回可用金额、实例内资金变更版本；没有公开 lookup、allocationView、resolveAllocation 或 UTXO 枚举，不返回数量或确认状态金额分栏。

本地找零在 WoC 首次报告前保留；报告后交给完整 WoC 观察更新。当前实例已取走的输入不会被旧 WoC 响应复活。实例销毁后全部丢弃，重启不补偿冻结。

钱包输入只通过 request / requestAll 分配，不支持调用者指定输入补冻结。纯协议池输入不创建钱包分配，成功返回钱包的输出仍须回填。

sendAll 先 requestAll，再由业务根据已冻结输入计算费用及无找零收款金额；费用不足、构建或预览保存失败释放原 requestId（即 allocationId），明确未派发／拒绝先封住业务提交再释放，广播前签名错误（含结果未知）取消原提交后释放，迟到签名不得派发；广播未知不释放。requestAll 不返回交易金额／费用／找零，共用内部 AllocationView 的这三个字段置零，不能用来重建业务计划。

业务保存原交易计划与对应 exact 的回执；分配状态不代表该付款成功；输出注入不改变输入锁或分配状态，不感知已花费，成功后不得释放由调用者保证。manager 不接收业务提交编号或付款状态。

版本：npm `keymaster-utxo-manager@0.4.0`，Go 根标签 `v0.4.0`。Go 标签与 npm 包独立发布，npm 是否已公开以 registry 为准。宿主使用正式发布的精确版本；源码、候选包与正式依赖分别验证，发布不代表宿主已切换或网站、服务已部署。

使用说明见各语言目录；当前契约、宿主接入与公共测试只维护在同级工程仓 `BSV8_Engineering/docs/utxo-manager/` 和 `parity/utxo-manager/`。`setDebugSink` / `SetDebugSink` 可开启脱敏内存诊断，诊断不成为资金事实。
