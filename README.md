# Fiction Factory (ff)

从中文网络热点出发，用「导演 + 主角」两个 AI Agent 双向互验、自动生成整本小说的 Go CLI。

不是「给个提示词直接出全文」：ff 维护一个持续演化的故事世界（世界事实 / 伏笔线程 / 角色信念），每章由**事件驱动角色成长**，完稿后自动跑一致性检查（角色漂移 / 时间线 / 伏笔）。

## 特性

- 📡 **热点驱动**：微博热搜 → 知乎热榜 → 百度热搜，自动降级；AI 把热点转化为原创小说前提
- 🎬 **双 Agent 互验**：导演提案事件 → 主角校验（不通过则导演修订）；主角心理反应 → 导演校验（不通过则主角修订），最多 N 轮后强制接受
- 📖 **结构化产出**：每个事件带三大支柱（facts_changed / belief_changes / future_hooks）+ 因果图，正文导出 markdown
- 🔍 **完稿质检**：角色漂移 / 时间线矛盾 / 被遗忘的伏笔，按 ERROR / WARNING / INFO 分级报告
- 🔌 **多 LLM 后端**：DeepSeek / Claude / Ollama / Mock（离线跑通全流程）

## 安装

```bash
go build -o ff ./cmd/ff
mv ff ~/.local/bin/
```

Go 版本要求见 `go.mod`。

## 快速开始

```bash
export DEEPSEEK_API_KEY="..."   # 或 ANTHROPIC_API_KEY；Ollama 无需 key

ff run                          # 全自动：爬热点 + 生成 10 章小说
ff run --list-topics            # 只看实时热点榜
ff run --topic 人工智能         # 用匹配关键词的热点
ff run --no-crawl --topic 外卖骑手  # 跳过爬取，用给定主题
ff run --chapters 20 --rounds 3 # 20 章，互验修订上限 3 轮
```

无项目时 `ff run` 自动完成「爬热点 → 生成前提 → 脚手架项目」，无需手动 init。

## 命令参考

| 命令 | 说明 |
|---|---|
| `ff init <name>` | 初始化项目（`-i` 交互向导，可配 genre/premise/POV/语言/provider） |
| `ff show` | 查看世界状态、角色信念/目标/恐惧、时间线与伏笔（`--detail` / `--hooks`） |
| `ff run` | 一键生成整本小说（`--chapters` `--rounds` `--topic` `--genre` `--title` `--language`） |
| `ff check` | 运行一致性检查（角色漂移 / 时间线 / 伏笔） |

全局参数：`-p/--project` 项目目录（默认当前目录）、`-v/--verbose`。

## 项目结构

```
project.yaml                  # 故事设定 + LLM 配置
world.yaml                    # 世界事实 / 伏笔线程 / 章节计数 / 叙事时间
characters/protagonist.yaml   # 主角信念、目标、恐惧、价值观、记忆
timeline/chapter-NN/evt-*.yaml  # 每个事件（三大支柱 + 因果图 + 主角反应）
generated/chapter-NN.md       # 章节正文（markdown）
```

`project.yaml` 的 `llm` 段配置 provider / model / api_key / base_url / temperature / max_tokens。

## 生成循环

```
📡 爬热点（微博 → 知乎 → 百度）
   └─ 💡 AI 生成前提（题目 / 梗概 / 主角初始设定）
        └─ 逐章循环：
             🎬 导演提案事件（七阶段分析 → selected_event）
                └─ 🎭 主角校验：心理一致性 / 世界事实 / 叙事逻辑
                     └─ 不通过 → 导演修订 → 再校验（最多 N 轮，超限强制接受）
             🎭 主角心理反应（情绪 / 决定 / 信念增改弃 / 记忆）
                └─ 🎬 导演校验 → 不通过 → 主角修订
             📝 生成正文 → 💾 保存
🔍 完稿后自动一致性检查
```

## 测试

```bash
go test ./...
```

Mock LLM 按提示词关键字返回合法桩，可离线跑通「热点 → 前提 → 互验循环 → 完稿检查」全流程。

## 相关项目

本仓库是 Fiction Factory 的 Go 原版 CLI。已有官方 DeepSeek Harness 移植版（`@deepseek-ai/dsh-fiction-factory`，`/novel` 命令 + 后台 job），由本仓库完全重构而来，模块一一对应。
