package conflicts

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
	"github.com/jesseduffield/lazygit/pkg/integration/tests/shared"
)

var ConflictMarkerSizeOtherFilesUnaffected = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Keeps the default conflict markers for files that a conflict-marker-size gitattribute doesn't cover",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig:  func(config *config.AppConfig) {},
	SetupRepo: func(shell *Shell) {
		shell.
			CreateFileAndAdd(".gitattributes", "special conflict-marker-size=32\n").
			Commit("set a conflict marker size for another file")
		shared.CreateMergeConflictFileMultiple(shell)
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
				Contains("<<<<<<< HEAD"),
				Contains("First Change"),
				Contains("======="),
			).
			PressPrimaryAction().
			SelectedLines(
				Contains("<<<<<<< HEAD"),
				Contains("Other First Change"),
				Contains("======="),
			).
			PressPrimaryAction()

		t.Common().ContinueOnConflictsResolved("merge")

		t.Views().Files().IsEmpty()
	},
})
