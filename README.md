# git-rewrite-authors-and-resign

**Danger**: This rewrites all commit hashes. You will have to force-push.

**Really**: You can break your repository if you don’t know what you’re doing.

## Binaries

A set of tools for rewriting git history and re-signing commits, written in Go.

This repository provides two standalone binaries:

- `change-name`: Rewrites authors/committers/Signed-off-by lines by streaming git history through
  git fast-export -> transform -> git fast-import.Rewrites authors/committers/Signed-off-by
- `re-sign`: Rebuilds commit history and re-signs every commit using `git commit-tree -S`, preserving dates and parent structure.

### change-name
Rewrites:

- `author` name/email
- `committer` name/email
- `Signed-off-by` lines
- rewrites commit message blocks cleanly
- optionally re-signs commits (but this step is now also available as its own tool)

### re-sign

A standalone commit resigning utility.
This binary only re-signs existing commits without rewriting emails/names.

Useful when:
- You already rewrote history (or imported someone else's)
- You want to add/replace commit signatures
- You want the signing step separated for clarity or safety
- You’re using SSH signing, GPG, or other signature configurations

## usage

### change-name

**Basic example**:

```bash
git-change-name \
  -oldName "Old Dev" \
  -oldEmail old@example.com \
  -newName "New Dev" \
  -newEmail new@example.com
```

**With built-in re-signing**:

```bash
git-change-name \
  -oldName "Old" \
  -oldEmail old@example.com \
  -newName "New" \
  -newEmail new@example.com \
  -signCommits \
  -signOnBranch re-signed
```

### re-sign

This tool walks the DAG (oldest -> newest), reconstructs each commit using
`git commit-tree -S`, and creates a parallel fully signed commit chain.

**Default usage**:

```bash
git-re-sign
```

## License

Copyright 2025 itsrye.dev

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.