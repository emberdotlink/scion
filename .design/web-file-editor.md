# Web-Based File Editor

**Status:** Draft
**Created:** 2026-04-03
**Related:** [web-frontend-design.md](./hosted/web-frontend-design.md), [template-editor.md](./template-editor.md), [web-full-config-create.md](./web-full-config-create.md)

---

## 1. Overview

### Goal

Add an inline file editor to the web UI that supports editing raw text formats (markdown, JSON, YAML, TOML, shell scripts, Go, TypeScript, etc.) with syntax highlighting. The editor is a shared component consumed by both the workspace/shared-dir file browser and the template editor.

### Current State

The workspace file browser (`grove-detail.ts`) supports file listing, upload, download, delete, and external preview (opens a new browser tab via `?view=true`). There is no inline viewing or editing. Clicking the eye icon opens the raw file in a new tab. There is no pencil/edit icon.

### Scope

This document covers the editor component itself, its integration into the existing workspace file browser, and the API changes needed to support reading and writing file content. The template editor integration is documented separately in [template-editor.md](./template-editor.md).

---

## 2. User Experience

### 2.1 Opening the Editor

**Workspace / Shared-Dir File Browser:**

- A new **pencil icon** (`pencil`) is added to each file row's action column, between the existing eye (preview) and download icons.
- Clicking the pencil opens the file in the inline editor panel.
- The pencil icon is shown for all text-editable file types (see Section 2.4). For non-editable types (images, PDFs, binaries), the pencil is hidden or disabled.

**Markdown Preview (Eye Icon):**

- For `.md` files, the eye icon opens the editor in **preview mode** — rendering the markdown as formatted HTML rather than opening a new tab.
- For non-markdown previewable files (images, PDFs), the eye icon retains its current behavior (new tab).

### 2.2 Editor Panel Layout

The editor opens as a panel within the existing page, replacing or overlaying the file list. Two layout approaches to consider:

**Option A: Slide-Over Panel (Recommended)**
```
┌─────────────────────────────────────────────────────────┐
│ File Browser (dimmed/narrowed)  │  Editor Panel         │
│                                 │ ┌───────────────────┐ │
│  file-list...                   │ │ toolbar            │ │
│                                 │ │ [Save] [Close] [P]│ │
│                                 │ ├───────────────────┤ │
│                                 │ │                   │ │
│                                 │ │  editor content   │ │
│                                 │ │                   │ │
│                                 │ └───────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

- File browser remains visible but narrowed. User can click another file to switch.
- Natural for browsing + editing workflows.

**Option B: Full-Width Replacement**
```
┌─────────────────────────────────────────────────────────┐
│ ← Back to files    filename.md              [Save] [P]  │
│┌───────────────────────────────────────────────────────┐│
││                                                       ││
││  editor content                                       ││
││                                                       ││
│└───────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────┘
```

- Simpler implementation. Maximizes editor real estate.
- "Back" button returns to file list.

**Recommendation:** Start with Option B (simpler) and evolve to Option A if multi-file workflows prove common.

### 2.3 Editor Toolbar

```
[filename.yaml]  [Save] [Revert] [Preview]  [Close ✕]
```

- **Filename** — displayed as a breadcrumb (read-only label).
- **Save** — writes changes back to the server. Disabled when no unsaved changes.
- **Revert** — discards unsaved changes and reloads from server. Confirm if dirty.
- **Preview** — (markdown only) toggles between edit and rendered preview. Icon: `eye` / `code`.
- **Close** — returns to file list. Prompts if unsaved changes exist.
- **Dirty indicator** — a dot or asterisk next to the filename when unsaved changes exist.

### 2.4 Supported File Types

| Category | Extensions | Syntax Mode | Editable | Preview |
|----------|-----------|-------------|----------|---------|
| Markdown | `.md` | markdown | Yes | Yes (rendered HTML) |
| JSON | `.json` | json | Yes | No |
| YAML | `.yaml`, `.yml` | yaml | Yes | No |
| TOML | `.toml` | toml | Yes | No |
| Shell | `.sh`, `.bash`, `.zsh` | shell | Yes | No |
| Go | `.go` | go | Yes | No |
| TypeScript/JS | `.ts`, `.tsx`, `.js`, `.jsx` | typescript/javascript | Yes | No |
| Python | `.py` | python | Yes | No |
| Rust | `.rs` | rust | Yes | No |
| HTML | `.html`, `.htm` | html | Yes | No |
| CSS | `.css`, `.scss` | css | Yes | No |
| Plain Text | `.txt`, `.log`, `.csv`, `.env`, `.gitignore` | plaintext | Yes | No |
| Dockerfile | `Dockerfile` | dockerfile | Yes | No |
| Images | `.png`, `.jpg`, `.gif`, `.svg`, `.webp` | — | No | Yes (inline) |
| PDF | `.pdf` | — | No | Yes (new tab) |
| Binary | everything else | — | No | No |

Detection is by file extension, with fallback to `text/plain` for unknown extensions if the file content is valid UTF-8.

---

## 3. Syntax Highlighting — Library Selection

### 3.1 Alternatives Considered

| Library | Size (min+gz) | Languages | Editing | Notes |
|---------|--------------|-----------|---------|-------|
| **CodeMirror 6** | ~150KB | 30+ via packages | Full editor | Modular, tree-sitter-like parsing, excellent accessibility. Active development. |
| **Monaco Editor** | ~2MB+ | 50+ | Full editor | VS Code's engine. Extremely powerful but very heavy. Overkill for this use case. |
| **Highlight.js** | ~40KB + langs | 190+ | Read-only | Lightweight, great coverage. No editing support — requires pairing with a textarea. |
| **Prism.js** | ~20KB + langs | 290+ | Read-only | Similar to highlight.js. No editing. |
| **Shiki** | ~1MB (WASM) | 100+ | Read-only | VS Code-quality highlighting using TextMate grammars. Heavy due to WASM/Oniguruma. |

### 3.2 Recommendation: CodeMirror 6

**Rationale:**
- Provides both syntax highlighting AND editing in one package — no need to pair a highlighter with a separate editor.
- Modular architecture lets us ship only the language modes we need, keeping bundle size reasonable (~150KB for core + a handful of languages).
- Excellent keyboard accessibility and screen reader support.
- Active ecosystem with good Lit/Web Component integration examples.
- Used widely in production (Observable, Replit, Firefox DevTools).

**Trade-offs:**
- Heavier than highlight.js for read-only use cases. Acceptable since we need editing.
- Learning curve for extensions/state model. Mitigated by good documentation.

### 3.3 Alternative: Highlight.js + Textarea (Simpler Approach)

If we want to minimize dependencies, a lighter approach is:

1. Use `highlight.js` for syntax-colored read-only view.
2. Switch to a plain `<textarea>` on edit, losing syntax highlighting during editing.
3. Re-highlight on save/preview.

**Pros:** Much smaller bundle. Simpler to integrate.
**Cons:** Poor editing experience (no syntax colors while typing, no bracket matching, no auto-indent). Users accustomed to modern editors will find this jarring.

**Verdict:** Not recommended unless bundle size is a hard constraint.

---

## 4. Markdown Preview

### 4.1 Rendering

Use a client-side markdown renderer. Options:

| Library | Size | Features | Notes |
|---------|------|----------|-------|
| **marked** | ~10KB | Fast, CommonMark-ish | Mature, widely used. Needs sanitization. |
| **markdown-it** | ~30KB | CommonMark + plugins | Extensible, good plugin ecosystem. |
| **micromark** | ~15KB | Strict CommonMark | Small, spec-compliant. Less extensible. |

**Recommendation:** `marked` — lightweight, fast, well-known. Pair with `DOMPurify` (~7KB) for XSS sanitization of rendered HTML.

### 4.2 Preview Modes

- **Toggle mode** (default): Toolbar button switches between editor and preview in the same space.
- **Side-by-side mode** (stretch goal): Split pane with editor on left, preview on right. Synchronized scrolling.

### 4.3 Eye Icon Behavior Change

Currently the eye icon for all previewable files opens a new browser tab. The proposed change:

| File Type | Eye Icon Behavior (Current) | Eye Icon Behavior (Proposed) |
|-----------|-----------------------------|------------------------------|
| `.md` | New tab (raw text) | **Inline markdown preview** |
| Images | New tab | New tab (unchanged) |
| Code files | New tab (raw text) | New tab (unchanged) |
| PDF | New tab | New tab (unchanged) |

Only markdown changes behavior. This is a targeted enhancement — the eye icon becomes contextual for markdown files.

---

## 5. API Changes

### 5.1 Read File Content (New Endpoint)

The existing download endpoint (`GET .../workspace/files/{path}`) returns the file as a download or inline view. For the editor, we need the content as a JSON-wrapped response with metadata:

**Option A: New dedicated endpoint**
```
GET /api/v1/groves/{groveId}/workspace/files/{filePath}/content
```
Response:
```json
{
  "path": "README.md",
  "content": "# Hello\n\nThis is content...",
  "size": 1234,
  "modTime": "2026-04-03T12:00:00Z",
  "encoding": "utf-8"
}
```

**Option B: Query parameter on existing endpoint**
```
GET /api/v1/groves/{groveId}/workspace/files/{filePath}?format=json
```

Same response body.

**Recommendation:** Option B — avoids adding a new route. The `format=json` param is consistent with the existing `view=true` pattern.

### 5.2 Write File Content (New Endpoint)

```
PUT /api/v1/groves/{groveId}/workspace/files/{filePath}
Content-Type: application/json

{
  "content": "# Updated content\n...",
  "expectedModTime": "2026-04-03T12:00:00Z"  // optional optimistic concurrency
}
```

- Creates the file if it doesn't exist, overwrites if it does.
- `expectedModTime` enables conflict detection: if the server's file has been modified since the client loaded it, return `409 Conflict`.
- Returns the updated file metadata on success.

**Alternative:** Use the existing upload endpoint with `Content-Type: text/plain` body instead of multipart form. This avoids a new endpoint but changes the upload contract.

### 5.3 File Size Limit for Editing

Files above a threshold (e.g., 1MB) should not be opened in the editor — the browser will struggle with large text in CodeMirror. The UI should show a warning and offer download instead.

---

## 6. Component Architecture

### 6.1 Component Hierarchy

```
scion-file-editor (LitElement)
├── toolbar (save, revert, preview toggle, close)
├── scion-code-editor (wraps CodeMirror)
│   └── CodeMirror EditorView
└── scion-markdown-preview (conditional)
    └── rendered HTML (via marked + DOMPurify)
```

### 6.2 `scion-file-editor`

Top-level component managing the editing session.

**Properties:**
- `filePath: string` — path of the file being edited
- `groveId: string` — grove context
- `source: 'workspace' | 'shared-dir' | 'template'` — determines API base path
- `sourceId?: string` — shared-dir name or template ID
- `readonly: boolean` — disables editing (e.g., locked templates, read-only shared dirs)

**Events:**
- `file-saved` — emitted after successful save
- `editor-closed` — emitted when user closes the editor

### 6.3 `scion-code-editor`

Thin wrapper around CodeMirror 6.

**Properties:**
- `content: string` — initial content
- `language: string` — syntax mode name
- `readonly: boolean`

**Events:**
- `content-changed` — emitted on edits (debounced), carries current content

### 6.4 `scion-markdown-preview`

Renders markdown as sanitized HTML.

**Properties:**
- `content: string` — raw markdown text

---

## 7. Permissions & Safety

- **Edit capability** gated on the existing `update` capability for the grove/resource. If the user cannot upload/delete files, they also cannot edit.
- **Read-only mode** for shared dirs configured as read-only. The editor opens but the save button is hidden/disabled.
- **Path validation** reuses the existing `validateWorkspaceFilePath()` server-side logic — no `..` traversal, no `.scion/` prefix.
- **Unsaved changes warning** — prompt before navigating away or closing the editor if dirty.

---

## 8. Open Questions

1. **Bundle strategy** — CodeMirror 6 is modular. Should we lazy-load the editor chunk only when the user opens a file, or include it in the main bundle? Lazy-loading is preferable but adds complexity.

2. **Conflict resolution** — The `expectedModTime` optimistic locking approach is simple but coarse. If two users edit the same file, the second saver gets a 409. Should we show a diff, or just force the user to reload? For MVP, reload + retry is sufficient.

3. **New file creation** — Should the editor support creating new files from scratch, or only editing existing ones? The file browser currently supports upload but not "new file." Adding a "New File" button is a natural extension but can be deferred.

4. **Tab/multi-file editing** — Should the editor support opening multiple files in tabs? This is common in IDEs but adds significant complexity. Recommendation: defer, start with single-file.

5. **Auto-save** — Should we auto-save drafts to localStorage to survive accidental page closes? Useful but adds complexity. Could be a Phase 2 feature.

6. **Max file size for editing** — 1MB is proposed as the cutoff. Is this appropriate? CodeMirror handles multi-MB files reasonably well, but network transfer and JSON encoding add overhead.

7. **Image/binary preview inline** — Should non-editable files (images) show an inline preview in the same panel? Or keep the current new-tab behavior? Inline preview is nicer but orthogonal to the editor feature.

---

## 9. Implementation Phases

### Phase 1: Core Editor (MVP)

- [ ] Add `scion-code-editor` component wrapping CodeMirror 6 with basic language modes (markdown, JSON, YAML, shell, Go, TypeScript)
- [ ] Add `scion-file-editor` component with toolbar (save, revert, close)
- [ ] Add `PUT` endpoint for writing file content
- [ ] Add `?format=json` support to existing download endpoint
- [ ] Add pencil icon to workspace file browser rows
- [ ] Full-width replacement layout (Option B)
- [ ] Gate on `update` capability

### Phase 2: Markdown Preview

- [ ] Add `scion-markdown-preview` component with `marked` + `DOMPurify`
- [ ] Add preview toggle button to editor toolbar for `.md` files
- [ ] Change eye icon behavior for `.md` files to open inline preview
- [ ] Side-by-side split view (stretch)

### Phase 3: Polish & Extensions

- [ ] Lazy-load editor bundle
- [ ] Additional language modes (Python, Rust, CSS, HTML, Dockerfile)
- [ ] "New File" creation flow
- [ ] Auto-save to localStorage
- [ ] Read-only shared-dir support
- [ ] File size limit enforcement with graceful fallback

---

## 10. Dependencies

- **CodeMirror 6**: `@codemirror/view`, `@codemirror/state`, `@codemirror/commands`, `@codemirror/language`, plus per-language packages
- **marked**: Markdown rendering
- **DOMPurify**: HTML sanitization
- **Shoelace**: Existing UI library (icons: `pencil`, `eye`, `code`, `floppy`, `arrow-counterclockwise`)

New Shoelace icons required: `pencil`, `floppy`, `arrow-counterclockwise`, `code`. These must be added to `USED_ICONS` in `web/scripts/copy-shoelace-icons.mjs`.
