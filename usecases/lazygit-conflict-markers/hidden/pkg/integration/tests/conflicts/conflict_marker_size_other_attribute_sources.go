package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
)

// The marker lengths come from a .gitattributes file in a subdirectory and from .git/info/attributes, not from
// the repository's root .gitattributes.
var ConflictMarkerSizeOtherAttributeSources = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Honors conflict-marker-size attributes from a subdirectory .gitattributes and from .git/info/attributes",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig: func(config *config.AppConfig) {
		config.GetUserConfig().Gui.ShowFileTree = false
	},
	SetupRepo: func(shell *Shell) {
		shell.
			CreateFileAndAdd("docs/.gitattributes", "guide.txt conflict-marker-size=20\n").
			Commit("set a conflict marker size in a subdirectory").
			CreateFile(".git/info/attributes", "info.txt conflict-marker-size=16\n").
			NewBranch("branch1").
			CreateFileAndAdd("docs/guide.txt", "step: original\n").
			CreateFileAndAdd("info.txt", "note: original\n").
			Commit("original").
			UpdateFileAndAdd("docs/guide.txt", "step: first\n").
			UpdateFileAndAdd("info.txt", "note: first\n").
			Commit("first change").
			NewBranchFrom("branch2", "HEAD^").
			UpdateFileAndAdd("docs/guide.txt", "step: second\n").
			UpdateFileAndAdd("info.txt", "note: second\n").
			Commit("second change").
			Checkout("branch1").
			RunCommandExpectError([]string{"git", "merge", "--no-edit", "branch2"})
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Common().PretendMergeOrRebaseStartedInLazygit()

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU docs/guide.txt").IsSelected(),
				Contains("UU info.txt"),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(strings.Repeat("<", 20)+" HEAD"),
				Contains("step: first"),
				Contains(strings.Repeat("=", 20)),
			).
			PressPrimaryAction()

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU info.txt").IsSelected(),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(strings.Repeat("<", 16)+" HEAD"),
				Contains("note: first"),
				Contains(strings.Repeat("=", 16)),
			).
			PressPrimaryAction()

		t.Common().ContinueOnConflictsResolved("merge")

		t.Views().Files().IsEmpty()
	},
})
