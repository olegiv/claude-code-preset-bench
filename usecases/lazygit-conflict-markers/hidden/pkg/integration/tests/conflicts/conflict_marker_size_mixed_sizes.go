package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
)

// One merge leaves two conflicted files with different marker lengths, one of them with a space in its name:
// each must keep its own.
var ConflictMarkerSizeMixedSizes = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Resolves a merge whose conflicted files have different conflict marker lengths",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig: func(config *config.AppConfig) {
		config.GetUserConfig().Gui.ShowFileTree = false
	},
	SetupRepo: func(shell *Shell) {
		shell.
			CreateFileAndAdd(".gitattributes", "*.md conflict-marker-size=12\n").
			Commit("set a conflict marker size for markdown files").
			NewBranch("branch1").
			CreateFileAndAdd("file", "value: original\n").
			CreateFileAndAdd("my notes.md", "status: original\n").
			Commit("original").
			UpdateFileAndAdd("file", "value: first\n").
			UpdateFileAndAdd("my notes.md", "status: first\n").
			Commit("first change").
			NewBranchFrom("branch2", "HEAD^").
			UpdateFileAndAdd("file", "value: second\n").
			UpdateFileAndAdd("my notes.md", "status: second\n").
			Commit("second change").
			Checkout("branch1").
			RunCommandExpectError([]string{"git", "merge", "--no-edit", "branch2"})
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Common().PretendMergeOrRebaseStartedInLazygit()

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU file").IsSelected(),
				Contains("UU my notes.md"),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains("<<<<<<< HEAD"),
				Contains("value: first"),
				Contains("======="),
			).
			PressPrimaryAction()

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU my notes.md").IsSelected(),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(strings.Repeat("<", 12)+" HEAD"),
				Contains("status: first"),
				Contains(strings.Repeat("=", 12)),
			).
			PressPrimaryAction()

		t.Common().ContinueOnConflictsResolved("merge")

		t.Views().Files().IsEmpty()
	},
})
