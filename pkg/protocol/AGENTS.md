# Protocol Agent Rules

This package defines collaboration behavior. Keep protocol control logic language-agnostic.

## Hard rule: no content-pattern control logic

- Do **not** add runtime logic that inspects model text content using string matching, regex, keyword checks, or semantic heuristics to steer protocol behavior.
- Do **not** gate turns based on parsing natural-language phrasing (for example "duplicate question" detectors, canned-phrase detection, or tag stripping rules).
- Assume users may run protocols in any language.

## Allowed steering mechanisms

- Prompt design and system instructions.
- Turn/round sequencing.
- Tool budgets and tool policies.
- Participant ordering and role assignment.
- Session/runtime metadata.
- Turn-based style/collaboration mode rotation (for example per-round "extend/challenge/synthesize/reframe"), as long as it does not inspect content text.

## Structured outputs

- Prefer prompt-level behavior first.
- If structure is required, use explicit schema-constrained outputs (for example JSON objects/arrays) and keep validation schema-based, not language-content-based.
- Avoid hidden fallback logic that depends on natural-language pattern extraction.

## Review checklist for protocol changes

1. Can this behavior work regardless of user language?
2. Is steering done via prompts/rounds/budgets instead of text parsing?
3. Are tool/memory decisions controlled by policy and plan settings, not phrase detection?
4. Is any validation schema-level rather than content-pattern-level?
