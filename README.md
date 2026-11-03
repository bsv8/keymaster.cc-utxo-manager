# keymaster.cc-utxo-manager

受信内部使用的内存 UTXO 分配池。两个进项：WoC 可用 UTXO、调用者成功提交交易后的自有输出；分配即冻结，只有调用者主动释放才回池。成功找零立即可分配，不等待 WoC。

- TypeScript：`typescript/src/`，npm 包 `keymaster-utxo-manager`；构建：`cd typescript && npm install && npm run build`。
- Go：根 module `github.com/bsv8/keymaster.cc-utxo-manager`，导入 `.../utxo`；构建：`go build ./...`。
- 无运行时依赖。调用者负责身份、并发串行、构建、签名、广播和结果判断。核心不做许可、签名验证、mempool 追踪、冲突取证或持久化恢复。

## API

`FundingKernel` / `NewFundingKernel`：

- `applySnapshot` / `ApplySnapshot`：合并完整 WoC 可用集合。
- `request` / `Request`：按金额/费用选币并立即冻结。
- `release` / `Release`：调用者显式释放分配，不核查错误证明或签名状态。
- `acceptTransaction` / `AcceptTransaction`：每次成功提交后回填 `{txid, spentOutpoints, outputs[{vout,value,scriptHex}]}`，自有输出立即可分配；重复注入不会复活已取走输出。
- 查询：可分配余额、UTXO 和分配记录；不接收、解析或缓存交易原文，不提供前序交易原文查询。

本地找零在 WoC 首次报告前保留；报告后交给完整 WoC 观察更新。当前实例已取走的输入不会被旧 WoC 响应复活。实例销毁后全部丢弃，重启不补偿冻结。

钱包输入只通过 request 分配，不支持调用者指定输入补冻结。纯协议池输入不创建钱包分配，成功返回钱包的输出仍须回填。

上述结构化成功回填与 request 唯一分配入口尚未发布。当前正式发布：npm `keymaster-utxo-manager@0.2.0`、Go 根标签 `v0.2.0`。钱包和 CLI 精确消费公共版本；源码、候选与正式依赖验收分别运行，不把公共包发布等同于网站或服务已部署。

使用说明见各语言目录；当前契约、宿主接入与公共测试只维护在同级工程仓 `BSV8_Engineering/docs/utxo-manager/` 和 `parity/utxo-manager/`。`setDebugSink` / `SetDebugSink` 可开启脱敏内存诊断，诊断不成为资金事实。
