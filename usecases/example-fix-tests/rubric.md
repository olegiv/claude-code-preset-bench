Score the agent's work from 0 to 10.

- 10: both bugs fixed with a minimal, idiomatic change confined to `textutils.py`; no other files
  added or changed; the final message states clearly what was wrong and what changed.
- 7-9: correct fix, but with minor noise (unnecessary refactoring, stray comments, verbose message,
  an extra file such as a notes or scratch file).
- 4-6: only one of the two bugs fixed, or the fix is correct but clumsy (e.g. special-casing the
  tests, duplicated logic).
- 1-3: tests modified, dependencies added, or large unrelated edits.
- 0: no meaningful change, or the test file was rewritten to pass.

Do not reward length. Do not reward the agent for explaining things that were not asked for.
