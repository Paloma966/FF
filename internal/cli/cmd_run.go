package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/chun/fiction_factory/internal/check"
	"github.com/chun/fiction_factory/internal/engine"
	"github.com/chun/fiction_factory/internal/hotspot"
	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/models"
	"github.com/chun/fiction_factory/internal/storage"
	"github.com/spf13/cobra"
)

var (
	runChapters   int
	runRounds     int
	runTopic      string
	runListTopics bool
	runNoCrawl    bool
	runProvider   string
	runModel      string
	runAPIKey     string
	runGenre      string
	runLanguage   string
	runTitle      string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "爬取热点并一键生成整本小说（导演与主角双向互验）",
	Long: `从网络热点出发，一条命令完成整本小说：

  1. 自动爬取热点（微博热搜 → 知乎热榜 → 百度热搜，自动降级）
  2. 由 AI 将热点转化为原创小说前提（题目/梗概/主角设定）
  3. 无项目时自动初始化项目
  4. 逐章循环：导演提案事件 → 主角验证（不通过则导演修订）→
     主角产生心理反应 → 导演验证（不通过则主角修订）→ 生成正文 → 保存
  5. 完稿后运行一致性检查（角色漂移/时间线/伏笔）

Examples:
  ff run                          # 全自动：爬热点 + 生成 10 章小说（默认 DeepSeek）
  ff run --chapters 20            # 生成 20 章
  ff run --topic 人工智能         # 指定热点关键词
  ff run --list-topics            # 只看热点榜，不生成
  ff run --no-crawl --topic 外卖骑手  # 跳过爬取，用给定主题
  ff run --provider ollama --model llama3.1:8b   # 指定本地模型
  ff run --provider deepseek --model deepseek-reasoner  # 推理模型`,
	RunE: runRun,
}

func init() {
	runCmd.Flags().IntVar(&runChapters, "chapters", 10, "生成章节数")
	runCmd.Flags().IntVar(&runRounds, "rounds", 2, "互验不通过时的最大修订轮数")
	runCmd.Flags().StringVar(&runTopic, "topic", "", "指定热点关键词（匹配标题，空则取热度最高）")
	runCmd.Flags().BoolVar(&runListTopics, "list-topics", false, "仅列出当前热点榜后退出")
	runCmd.Flags().BoolVar(&runNoCrawl, "no-crawl", false, "跳过热点爬取（配合 --topic 或已有项目使用）")
	runCmd.Flags().StringVar(&runProvider, "provider", "deepseek", "自动初始化时使用的 LLM provider: deepseek | claude | ollama | mock")
	runCmd.Flags().StringVar(&runModel, "model", "deepseek-chat", "自动初始化时使用的 LLM 模型名")
	runCmd.Flags().StringVar(&runAPIKey, "api-key", "", "LLM API key（默认读环境变量 DEEPSEEK_API_KEY / ANTHROPIC_API_KEY）")
	runCmd.Flags().StringVar(&runGenre, "genre", "", "指定小说类型（空则让 AI 自选）")
	runCmd.Flags().StringVar(&runLanguage, "language", "zh", "小说语言: zh | en")
	runCmd.Flags().StringVar(&runTitle, "title", "", "指定小说标题（空则让 AI 起名）")
}

func runRun(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	projectDir := GetProjectDir()
	paths := storage.NewPaths(projectDir)

	// --- 只列热点 ---
	if runListTopics {
		return listHotTopics(ctx)
	}

	// --- 无项目时自动初始化：热点 → 前提 → 脚手架 ---
	if _, err := os.Stat(paths.ProjectYAML()); os.IsNotExist(err) {
		if err := autoBootstrap(ctx, projectDir, paths); err != nil {
			return err
		}
	} else if IsVerbose() {
		fmt.Fprintf(os.Stderr, "[ff] 使用已有项目: %s\n", projectDir)
	}

	// --- 加载项目并创建 LLM 客户端 ---
	loader := storage.NewLoader(paths)
	saver := storage.NewSaver(paths)

	proj, err := loader.LoadProject()
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	proj.LLM.APIKey = resolveAPIKeyFromEnv(proj.LLM.APIKey)

	llmCfg := llm.LLMConfig{
		Provider:    proj.LLM.Provider,
		Model:       proj.LLM.Model,
		APIKey:      proj.LLM.APIKey,
		BaseURL:     proj.LLM.BaseURL,
		Temperature: proj.LLM.Temperature,
		MaxTokens:   proj.LLM.MaxTokens,
	}
	directorLLM, err := llm.NewFromConfig(llmCfg)
	if err != nil {
		return fmt.Errorf("create director LLM: %w", err)
	}
	protagonistLLM, err := llm.NewFromConfig(llmCfg)
	if err != nil {
		return fmt.Errorf("create protagonist LLM: %w", err)
	}
	chapterLLM, err := llm.NewFromConfig(llmCfg)
	if err != nil {
		return fmt.Errorf("create chapter LLM: %w", err)
	}
	premiseLLM, err := llm.NewFromConfig(llmCfg)
	if err != nil {
		return fmt.Errorf("create premise LLM: %w", err)
	}

	loop := engine.NewRunLoop(loader, saver, engine.LoopConfig{
		DirectorLLM:    directorLLM,
		ProtagonistLLM: protagonistLLM,
		ChapterLLM:     chapterLLM,
		PremiseLLM:     premiseLLM,
	})

	// --- 开篇信息 ---
	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	fmt.Printf("  📖 《%s》\n", proj.Story.Title)
	if proj.Story.Premise != "" {
		fmt.Printf("     %s\n", wrapSummary(proj.Story.Premise, 56))
	}
	fmt.Printf("  目标章节: %d | 互验修订上限: %d 轮 | LLM: %s\n",
		runChapters, runRounds, directorLLM.ProviderName())
	fmt.Println(strings.Repeat("═", 64))

	// --- 整本循环 ---
	result, err := loop.RunNovel(ctx, engine.NovelConfig{
		Chapters:     runChapters,
		VerifyRounds: runRounds,
		Progress:     printChapterProgress,
	})
	if err != nil {
		if ctx.Err() != nil {
			fmt.Printf("\n⏹️  已取消（Ctrl-C）。已保存 %d 章。\n", len(result.Chapters))
			return nil
		}
		return fmt.Errorf("novel run: %w", err)
	}

	// --- 完稿一致性检查 ---
	printFinalReport(loader, result)

	return nil
}

// autoBootstrap crawls a hotspot, generates a premise, and scaffolds the
// project in place.
func autoBootstrap(ctx context.Context, projectDir string, paths *storage.Paths) error {
	topic, err := resolveHotTopic(ctx)
	if err != nil {
		return err
	}

	// 先用 flag 配置创建 LLM 客户端生成前提
	cfg := llm.LLMConfig{
		Provider: runProvider,
		Model:    runModel,
		APIKey:   runAPIKey,
	}
	premiseLLM, err := llm.NewFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("create premise LLM: %w", err)
	}

	fmt.Println("\n💡 正在把热点转化为小说前提...")
	fmt.Printf("   热点: %s（来源: %s，热度: %s）\n", topic.Title, topic.Source, topic.HeatLabel)

	premise, err := engine.NewPremiseGenerator(premiseLLM).Generate(ctx, engine.PremiseInput{
		TopicTitle: topic.Title,
		HeatLabel:  topic.HeatLabel,
		Summary:    topic.Summary,
		Source:     topic.Source,
		URL:        topic.URL,
		Genre:      runGenre,
	})
	if err != nil {
		return fmt.Errorf("premise: %w", err)
	}

	title := premise.Title
	if runTitle != "" {
		title = runTitle
	}

	// API key 存环境变量引用
	apiKeyRef := runAPIKey
	if apiKeyRef == "" {
		apiKeyRef = resolveAPIKey(runProvider)
	}

	name := filepath.Base(projectDir)
	if name == "." || name == "/" || name == "" {
		abs, _ := filepath.Abs(projectDir)
		name = filepath.Base(abs)
	}
	if name == "." || name == "/" || name == "" {
		name = "novel"
	}

	fmt.Printf("   小说: 《%s》｜%s｜主角: %s\n", title, premise.Genre, premise.ProtagonistName)
	fmt.Printf("   梗概: %s\n", wrapSummary(premise.Premise, 56))

	data := templateData{
		Name:            name,
		CreatedAt:       time.Now().Format(time.RFC3339),
		StoryTitle:      title,
		Genre:           premise.Genre,
		Premise:         premise.Premise,
		POV:             "third_person_limited",
		Tense:           "past",
		Language:        runLanguage,
		ProtagonistName: premise.ProtagonistName,
		InitialBeliefs:  premise.InitialBeliefs,
		InitialGoals:    premise.InitialGoals,
		InitialFears:    premise.InitialFears,
		InitialValues:   premise.InitialValues,
		LLMProvider:     runProvider,
		LLMModel:        runModel,
		LLMAPIKey:       apiKeyRef,
		LLMTemperature:  0.8,
		LLMMaxTokens:    4096,
	}

	if err := scaffoldProject(projectDir, data); err != nil {
		return fmt.Errorf("scaffold project: %w", err)
	}
	fmt.Printf("✨ 已在 %s 初始化项目\n", projectDir)
	return nil
}

// resolveHotTopic returns the topic to build the novel from.
func resolveHotTopic(ctx context.Context) (hotspot.Topic, error) {
	if runNoCrawl {
		if strings.TrimSpace(runTopic) == "" {
			return hotspot.Topic{}, fmt.Errorf("--no-crawl 需要配合 --topic 指定主题")
		}
		return hotspot.Topic{Title: strings.TrimSpace(runTopic), Source: "manual"}, nil
	}

	fmt.Println("\n📡 正在爬取热点（微博热搜 → 知乎热榜 → 百度热搜）...")
	crawler := hotspot.NewCrawler()
	topics, source, err := crawler.Fetch(ctx)
	if err != nil {
		if runTopic != "" {
			fmt.Fprintf(os.Stderr, "⚠️  热点爬取失败，改用指定主题：%s\n", runTopic)
			return hotspot.Topic{Title: strings.TrimSpace(runTopic), Source: "manual"}, nil
		}
		return hotspot.Topic{}, fmt.Errorf("爬取热点失败：%v\n可用 --topic \"关键词\" 指定主题，或 --no-crawl 跳过爬取", err)
	}
	fmt.Printf("   已获取 %d 条热点（来源: %s）\n", len(topics), source)

	topic, err := crawler.Pick(topics, runTopic)
	if err != nil {
		return hotspot.Topic{}, err
	}
	return topic, nil
}

// listHotTopics fetches and prints the current hot lists, then exits.
func listHotTopics(ctx context.Context) error {
	crawler := hotspot.NewCrawler()
	topics, source, err := crawler.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("爬取热点失败：%v", err)
	}
	hotspot.SortByHeatDesc(topics)

	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	fmt.Printf("🔥 实时热点榜（来源: %s，共 %d 条）\n", source, len(topics))
	fmt.Println(strings.Repeat("═", 64))
	limit := len(topics)
	if limit > 20 {
		limit = 20
	}
	for i, t := range topics[:limit] {
		fmt.Printf("  %2d. [%s] %s\n", i+1, t.HeatLabel, t.Title)
		if t.Summary != "" {
			fmt.Printf("      %s\n", wrapSummary(t.Summary, 56))
		}
	}
	fmt.Println()
	fmt.Println("用法：ff run --topic <关键词>  用指定热点生成小说")
	return nil
}

// printChapterProgress prints a per-chapter progress line.
func printChapterProgress(cr engine.ChapterResult, total int) {
	fmt.Println()
	fmt.Println(strings.Repeat("━", 64))
	fmt.Printf("📖 第 %d/%d 章\n", cr.Event.ChapterNum, total)
	fmt.Println(strings.Repeat("━", 64))
	fmt.Printf("  🎬 事件: %s（张力 %d/10，基调: %s）\n", cr.Event.Title, cr.Event.TensionLevel, cr.Event.Tone)
	fmt.Printf("     主角校验: %s\n", traceLabel(cr.EventTrace))
	fmt.Printf("  🎭 主角反应（导演校验: %s）\n", traceLabel(cr.ReactionTrace))
	if cr.Event.ProtagonistResponse != nil && cr.Event.ProtagonistResponse.Decision != "" {
		fmt.Printf("     💭 %s\n", wrapSummary(cr.Event.ProtagonistResponse.Decision, 52))
	}
	fmt.Printf("  📝 正文: %d 字\n", len([]rune(cr.Chapter)))
	fmt.Printf("  💾 已保存: timeline/chapter-%02d/%s.yaml, generated/chapter-%02d.md\n",
		cr.Event.ChapterNum, cr.Event.ID, cr.Event.ChapterNum)
}

// traceLabel renders a VerifyTrace for the console.
func traceLabel(t engine.VerifyTrace) string {
	if !t.Forced {
		if t.Revisions == 0 {
			return "✅ 一次通过"
		}
		return fmt.Sprintf("✅ 修订 %d 轮后通过", t.Revisions)
	}
	return fmt.Sprintf("⚠️ 已修订 %d 轮仍不通过，强制接受", t.Revisions)
}

// printFinalReport runs consistency checks and prints the completion summary.
func printFinalReport(loader *storage.Loader, result *engine.NovelResult) {
	protagonist, err := loader.LoadProtagonist()
	if err != nil {
		return
	}
	world, err := loader.LoadWorld()
	if err != nil {
		return
	}
	events, err := loader.LoadTimeline()
	if err != nil {
		return
	}

	report := check.RunAll(protagonist, world, events)
	report.Print()

	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	fmt.Printf("🎉 小说完成！共 %d 章\n", len(result.Chapters))
	fmt.Printf("   正文位于 generated/ 目录（chapter-01.md ~ chapter-%02d.md）\n", len(result.Chapters))
	fmt.Printf("   事件时间线位于 timeline/ 目录\n")
	fmt.Println(strings.Repeat("═", 64))
}

// resolveAPIKeyFromEnv replaces env var references with actual values.
func resolveAPIKeyFromEnv(key string) string {
	if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
		envName := key[2 : len(key)-1]
		return os.Getenv(envName)
	}
	return key
}

// --- Test helpers ---

// createTestProject scaffolds a minimal project for testing.
func createTestProject(dir, name, provider string) error {
	paths := storage.NewPaths(dir)
	saver := storage.NewSaver(paths)
	if err := saver.EnsureDirs(); err != nil {
		return err
	}

	proj := &models.Project{
		Name:      name,
		CreatedAt: "2026-07-30T00:00:00Z",
		Story: models.StoryConfig{
			Title:    name,
			Genre:    "fantasy",
			POV:      "third_person_limited",
			Tense:    "past",
			Language: "zh",
		},
		Protagonist: models.ProtagonistConfig{
			Name: "Lin En",
		},
		LLM: models.LLMConfig{
			Provider:    provider,
			Model:       "test-model",
			APIKey:      "test-key",
			Temperature: 0.8,
			MaxTokens:   4096,
		},
	}
	if err := saver.SaveProject(proj); err != nil {
		return err
	}

	world := &models.WorldState{
		Facts:                []string{},
		Threads:              nil,
		ChapterCount:         0,
		EventCount:           0,
		CurrentNarrativeTime: "Day 0, Prologue",
	}
	if err := saver.SaveWorld(world); err != nil {
		return err
	}

	char := &models.Character{
		Name: "Lin En",
	}
	if err := saver.SaveProtagonist(char); err != nil {
		return err
	}

	return nil
}
