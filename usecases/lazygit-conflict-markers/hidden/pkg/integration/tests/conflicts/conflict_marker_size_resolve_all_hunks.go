package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
	"github.com/jesseduffield/lazygit/pkg/integration/tests/shared"
)

var ConflictMarkerSizeResolveAllHunks = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Resolves every hunk of a file with longer conflict markers and is then offered to continue the merge",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig:  func(config *config.AppConfig) {},
	SetupRepo: func(shell *Shell) {
		shared.SetCustomConflictMarkerSize(shell)
		shared.CreateMergeConflictFileMultiple(shell)
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Common().PretendMergeOrRebaseStartedInLazygit()

		startMarker := strings.Repeat("<", shared.CustomConflictMarkerSize)
		separator := strings.Repeat("=", shared.CustomConflictMarkerSize)

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU file").IsSelected(),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(startMarker+" HEAD"),
				Contains("First Change"),
				Contains(separator),
			).
			PressPrimaryAction().
			SelectedLines(
				Contains(startMarker+" HEAD"),
				Contains("Other First Change"),
				Contains(separator),
			).
			PressPrimaryAction()

		t.Common().ContinueOnConflictsResolved("merge")

		t.Views().Files().IsEmpty()
	},
})
