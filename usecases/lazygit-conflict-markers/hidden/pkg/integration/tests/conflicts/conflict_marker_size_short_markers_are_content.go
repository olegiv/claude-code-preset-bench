package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
	"github.com/jesseduffield/lazygit/pkg/integration/tests/shared"
)

// A file that documents conflict markers contains a default-length marker block as ordinary text.
// With a longer conflict-marker-size, only the long markers delimit the real conflict.
const exampleMarkerBlock = "<<<<<<< ours\nour side\n=======\ntheir side\n>>>>>>> theirs\n"

var ConflictMarkerSizeShortMarkersAreContent = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Treats default-length marker lines as content in a file whose conflict-marker-size gitattribute is longer",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig:  func(config *config.AppConfig) {},
	SetupRepo: func(shell *Shell) {
		shared.SetCustomConflictMarkerSize(shell)
		shell.
			NewBranch("branch1").
			CreateFileAndAdd("file", "How to read a conflict:\n"+exampleMarkerBlock+"value: original\n").
			Commit("original").
			UpdateFileAndAdd("file", "How to read a conflict:\n"+exampleMarkerBlock+"value: first\n").
			Commit("first change").
			NewBranchFrom("branch2", "HEAD^").
			UpdateFileAndAdd("file", "How to read a conflict:\n"+exampleMarkerBlock+"value: second\n").
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
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(strings.Repeat("<", shared.CustomConflictMarkerSize)+" HEAD"),
				Contains("value: first"),
				Contains(strings.Repeat("=", shared.CustomConflictMarkerSize)),
			).
			PressPrimaryAction()

		t.Common().ContinueOnConflictsResolved("merge")

		t.FileSystem().FileContent("file", Equals("How to read a conflict:\n"+exampleMarkerBlock+"value: first\n"))
	},
})
