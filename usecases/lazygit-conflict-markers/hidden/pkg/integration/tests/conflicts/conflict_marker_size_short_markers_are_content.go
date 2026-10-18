package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
	"github.com/jesseduffield/lazygit/pkg/integration/tests/shared"
)

// A file that documents conflict markers contains marker-like blocks as ordinary text: one of the default length
// and one a character longer than the file's conflict-marker-size. Only markers of exactly that size delimit the
// real conflict.
var exampleMarkerBlocks = "How to read a conflict:\n" +
	"<<<<<<< ours\nour side\n=======\ntheir side\n>>>>>>> theirs\n" +
	strings.Repeat("<", shared.CustomConflictMarkerSize+1) + " ours\nour side\n" +
	strings.Repeat("=", shared.CustomConflictMarkerSize+1) + "\ntheir side\n" +
	strings.Repeat(">", shared.CustomConflictMarkerSize+1) + " theirs\n"

var ConflictMarkerSizeShortMarkersAreContent = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Treats marker lines of other lengths as content in a file whose conflict-marker-size gitattribute is longer",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig:  func(config *config.AppConfig) {},
	SetupRepo: func(shell *Shell) {
		shared.SetCustomConflictMarkerSize(shell)
		shell.
			NewBranch("branch1").
			CreateFileAndAdd("file", exampleMarkerBlocks+"value: original\n").
			Commit("original").
			UpdateFileAndAdd("file", exampleMarkerBlocks+"value: first\n").
			Commit("first change").
			NewBranchFrom("branch2", "HEAD^").
			UpdateFileAndAdd("file", exampleMarkerBlocks+"value: second\n").
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

		t.FileSystem().FileContent("file", Equals(exampleMarkerBlocks+"value: first\n"))
	},
})
