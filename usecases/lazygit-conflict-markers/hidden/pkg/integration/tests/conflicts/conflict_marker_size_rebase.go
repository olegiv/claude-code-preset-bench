package conflicts

import (
	"strings"

	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
	"github.com/jesseduffield/lazygit/pkg/integration/tests/shared"
)

// A rebase started in lazygit, not a merge, stops on a file with longer conflict markers.
var ConflictMarkerSizeRebase = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Resolves a rebase conflict in a file whose conflict-marker-size gitattribute makes its markers longer than usual",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig: func(config *config.AppConfig) {
		config.GetUserConfig().Git.LocalBranchSortOrder = "recency"
	},
	SetupRepo: func(shell *Shell) {
		shared.SetCustomConflictMarkerSize(shell)
		shared.MergeConflictsSetup(shell)
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Views().Branches().
			Focus().
			NavigateToLine(Contains("second-change-branch")).
			Press(keys.Branches.RebaseBranch)

		t.ExpectPopup().Menu().
			Title(Equals("Rebase 'first-change-branch'")).
			Select(Contains("Simple rebase")).
			Confirm()

		t.Common().AcknowledgeConflicts()

		t.Views().Files().
			IsFocused().
			Lines(
				Contains("UU file").IsSelected(),
			).
			// The file still has its conflicts, so a refresh must not stage it
			Press(keys.Universal.Refresh).
			Lines(
				Contains("UU file").IsSelected(),
			).
			PressEnter()

		t.Views().MergeConflicts().
			IsFocused().
			SelectedLines(
				Contains(strings.Repeat("<", shared.CustomConflictMarkerSize)+" HEAD"),
				Contains("Second Change"),
				Contains(strings.Repeat("=", shared.CustomConflictMarkerSize)),
			).
			PressPrimaryAction()

		t.Views().Information().Content(Contains("Rebasing"))

		t.Common().ContinueOnConflictsResolved("rebase")

		t.Views().Information().Content(DoesNotContain("Rebasing"))
	},
})
