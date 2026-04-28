# destroy

## Contract

- Command: `idx destroy`
- Purpose: Remove index metadata from project.
- Scope: `.idx` data under project root.

## Parameters

- Positional args: none
- Flags: none

## Preconditions

- Must run from Git project root.

## Behavior

- Removes index metadata recursively.

## Side effects

- Deletes `.idx` content used by indexing/search.

## Success output

- `🧹 Index metadata removed from project.`

## Common failures

- Running outside project root.
- Permission errors when removing files.
