package cli

import (
	"fmt"
	"os"

	"github.com/chun/fiction_factory/internal/check"
	"github.com/chun/fiction_factory/internal/storage"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "对故事世界运行一致性检查",
	Long: `从三个维度校验故事世界的内部一致性：

  角色漂移:
    检测无动机的信念反转、无原因的性格漂移，
    以及活跃信念与已抛弃信念之间的冲突。

  时间线:
    按章节检查事件顺序，发现时间矛盾，
    并标记主角缺席且无解释的事件。

  被遗忘的伏笔:
    识别被搁置的未解决钩子、标记为 "immediate" 却迟迟未兑现的钩子，
    以及重复的钩子 ID。

问题按严重程度分级报告：
  ❌ ERROR   — 必须修复（破坏连续性）
  ⚠️  WARNING — 应当复查（潜在问题）
  ℹ️  INFO    — 仅供参考（酌情处理）`,
	RunE: runCheck,
}

func runCheck(cmd *cobra.Command, args []string) error {
	projectDir := GetProjectDir()
	paths := storage.NewPaths(projectDir)

	if _, err := os.Stat(paths.ProjectYAML()); os.IsNotExist(err) {
		return fmt.Errorf("no project found at %s — run 'ff init' first", projectDir)
	}

	loader := storage.NewLoader(paths)

	protagonist, err := loader.LoadProtagonist()
	if err != nil {
		return fmt.Errorf("load protagonist: %w", err)
	}

	world, err := loader.LoadWorld()
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}

	events, err := loader.LoadTimeline()
	if err != nil {
		return fmt.Errorf("load timeline: %w", err)
	}

	report := check.RunAll(protagonist, world, events)
	report.Print()

	return nil
}
