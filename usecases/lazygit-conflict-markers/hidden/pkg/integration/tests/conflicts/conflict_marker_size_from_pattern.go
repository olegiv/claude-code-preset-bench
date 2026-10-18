package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
)

// The attribute comes from a glob pattern, with a size other than the one the other tests use.
const patternConflictMarkerSize = 12

var ConflictMarkerSizeFromPattern = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Honors a conflict-marker-size that a .gitattributes pattern assigns",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig:  func(config *config.AppConfig) {},
	SetupRepo: func(shell *Shell) {
		shell.
			CreateFileAndAdd(".gitattributes", "*.md conflict-marker-size=12\n").
			Commit("set a conflict marker size for markdown files").
			NewBranch("branch1").
			CreateFileAndAdd("notes.md", "# Notes\n\nstatus: original\n").
			Commit("original").
			UpdateFileAndAdd("notes.md", "# Notes\n\nstatus: first\n").
			Commit("first change").
			NewBranchFrom("branch2", "HEAD^").
			UpdateFileAndAdd("notes.md", "# Notes\n\nstatus: second\n").
			Commit("second change").
			Checkout("branch1").
			RunCommandExpectError([]string{"git", "merge", "--no-edit", "branch2"})
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Common().PretendMergeOrRebaseStartedInLazygit()

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU notes.md").IsSelected(),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(strings.Repeat("<", patternConflictMarkerSize)+" HEAD"),
				Contains("status: first"),
				Contains(strings.Repeat("=", patternConflictMarkerSize)),
			).
			SelectNextItem().
			PressPrimaryAction()

		t.Common().ContinueOnConflictsResolved("merge")

		t.FileSystem().FileContent("notes.md", Equals("# Notes\n\nstatus: second\n"))
	},
})
