# Template Viewer & Editor (Web UI)

**Status:** Draft
**Created:** 2026-04-03
**Related:** [web-file-editor.md](./web-file-editor.md), [hosted-templates.md](./hosted/hosted-templates.md), [agnostic-template-design.md](./agnostic-template-design.md), [grove-level-templates.md](./grove-level-templates.md)

---

## 1. Overview

### Goal

Enable browsing and editing template file contents directly in the web UI. Templates currently appear as metadata-only items in grove settings — users can see name, description, and harness type but cannot view or modify the actual files (CLAUDE.md, system prompts, config files, etc.) without downloading them externally.

This design re-purposes the existing workspace file browser component to display template contents, and integrates the shared file editor component ([web-file-editor.md](./web-file-editor.md)) for inline editing.

### Current State

- **Template listing** in grove settings (`grove-settings.ts`) shows name/description/harness badge in a flat list under the Resources > Templates tab.
- **No file browsing** — template files are only accessible via the download API which returns signed URLs. There is no UI to browse the file tree.
- **No inline editing** — template modifications require downloading files, editing locally, and re-uploading (or creating a new template version).
- **Template API** supports file listing via the download endpoint (`GET /api/v1/templates/{id}/download`) which returns a manifest with file paths, sizes, and hashes.

### Scope

This document covers:
- Expanding a template in grove settings to show a file browser
- Re-using the workspace file browser for template contents
- Integrating the file editor for template file editing
- API changes needed to support reading/writing individual template files
- Template versioning considerations

This document does NOT cover:
- The file editor component itself (see [web-file-editor.md](./web-file-editor.md))
- Template metadata editing (name, description, harness — already supported via PATCH)
- Template creation or import workflows (already implemented)

---

## 2. User Experience

### 2.1 Template Expansion in Grove Settings

Currently, clicking a template in the Resources > Templates list does nothing. The proposed change:

```
┌─────────────────────────────────────────────────────────┐
│ Resources  [Env Vars] [Secrets] [Shared Dirs] [Templates]│
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ▶ claude-default     Claude agent template    claude    │
│  ▼ my-custom-agent    Custom research agent    claude    │
│  ┌─────────────────────────────────────────────────────┐│
│  │ Template Files                        [↻] [Upload]  ││
│  │                                                     ││
│  │  📄 CLAUDE.md                1.2 KB   ✏️  👁  ⬇     ││
│  │  📄 home/.bashrc              340 B   ✏️  👁  ⬇     ││
│  │  📄 home/.config/settings.json 89 B   ✏️  👁  ⬇     ││
│  │  📄 system-prompt.md         2.1 KB   ✏️  👁  ⬇     ││
│  │                                                     ││
│  └─────────────────────────────────────────────────────┘│
│  ▶ data-analyst       Data analysis template   gemini   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Behavior:**
- Clicking a template row toggles expansion (accordion-style, one template expanded at a time).
- The expanded area shows the template's file tree using a re-purposed version of the workspace file browser component.
- File actions mirror the workspace browser: edit (pencil), preview (eye), download.
- Upload button allows adding new files to the template.
- Locked templates show a lock icon and disable edit/upload/delete actions.

### 2.2 File Browser Re-Use

The workspace file browser in `grove-detail.ts` renders a file table with columns: Name, Size, Modified, Actions. This same table structure is re-used for template files, with adaptations:

| Aspect | Workspace Browser | Template Browser |
|--------|------------------|------------------|
| Data source | Workspace filesystem API | Template file manifest + content API |
| File actions | Preview, Download, Delete, Edit | Preview, Download, Delete, Edit |
| Upload | Multipart file upload | Signed URL upload |
| Archive download | ZIP of workspace | ZIP of template (stretch) |
| Sorting | By name, size, modified | By name, size (no modTime on template files currently) |
| Path display | Relative to workspace root | Relative to template root |
| Permission gate | Grove `update` capability | Template `update` capability + not locked |

**Componentization approach:**

Extract the file table rendering from `grove-detail.ts` into a shared `scion-file-browser` component that accepts a data source adapter. Both the workspace page and the template expansion use this shared component with different adapters.

### 2.3 Editing Template Files

Clicking the pencil icon on a template file opens the shared `scion-file-editor` component (from [web-file-editor.md](./web-file-editor.md)).

**Key differences from workspace file editing:**
- **Save** writes back through the template file API (not the workspace API).
- **Versioning** — saving a template file may create a new template version (see Section 5).
- **Locked templates** — editor opens in read-only mode with a banner explaining the template is locked.
- **Scope awareness** — global templates may be viewable but not editable by grove-scoped users.

### 2.4 Navigation Flow

```
Grove Settings Page
  └── Resources > Templates tab
        └── Click template row → expand file browser
              └── Click pencil icon → open file editor
                    └── Save → write to template storage
                    └── Close → return to file browser
```

The editor replaces the template file browser content (same full-width replacement pattern as the workspace editor, per [web-file-editor.md](./web-file-editor.md)).

---

## 3. API Changes

### 3.1 Read Individual Template File Content

The current download endpoint returns signed URLs for all files. For inline editing, we need to fetch a single file's content directly.

**New endpoint:**
```
GET /api/v1/templates/{templateId}/files/{filePath}
```

Response:
```json
{
  "path": "CLAUDE.md",
  "content": "# My Agent\n\nSystem instructions...",
  "size": 1234,
  "hash": "sha256:abc123...",
  "encoding": "utf-8"
}
```

**Implementation:** The Hub fetches the file from storage (GCS/local) and returns the content inline. For cloud storage, this means the Hub proxies the content rather than redirecting to a signed URL — necessary because the browser editor needs the content as a JSON response, not a file download.

**Size limit:** Files above 1MB return `413 Payload Too Large` with a message suggesting download instead.

### 3.2 Write Individual Template File Content

**New endpoint:**
```
PUT /api/v1/templates/{templateId}/files/{filePath}
Content-Type: application/json

{
  "content": "# Updated content\n...",
  "expectedHash": "sha256:abc123..."  // optional optimistic concurrency
}
```

**Behavior:**
- Writes the file to template storage.
- Updates the template's file manifest (size, hash).
- Recomputes the template's `ContentHash`.
- If `expectedHash` is provided and doesn't match the current file hash, returns `409 Conflict`.
- Returns `403 Forbidden` for locked templates.

### 3.3 Delete Template File

**New endpoint:**
```
DELETE /api/v1/templates/{templateId}/files/{filePath}
```

- Removes file from storage and manifest.
- Returns `403` for locked templates.

### 3.4 Upload Template File

**New endpoint:**
```
POST /api/v1/templates/{templateId}/files
Content-Type: multipart/form-data
```

- Accepts one or more files.
- Adds to template storage and manifest.
- Alternative: continue using the existing signed-URL upload flow but expose it better in the UI.

### 3.5 Template File Listing

The existing download endpoint (`GET /api/v1/templates/{id}/download`) returns file metadata. For the file browser, we need a lighter listing endpoint that doesn't generate signed URLs:

**New endpoint:**
```
GET /api/v1/templates/{templateId}/files
```

Response:
```json
{
  "files": [
    { "path": "CLAUDE.md", "size": 1234, "hash": "sha256:abc..." },
    { "path": "home/.bashrc", "size": 340, "hash": "sha256:def..." }
  ],
  "totalSize": 1574,
  "totalCount": 2
}
```

This avoids the overhead of generating signed URLs when we just need to display the file tree.

---

## 4. Component Architecture

### 4.1 Shared File Browser Extraction

Extract from `grove-detail.ts` into a reusable component:

```
scion-file-browser (new shared component)
├── Properties:
│   files: FileEntry[]
│   loading: boolean
│   error: string | null
│   editable: boolean
│   sortField / sortDirection
├── Events:
│   file-edit-requested (filePath)
│   file-preview-requested (filePath)
│   file-download-requested (filePath)
│   file-delete-requested (filePath)
│   file-upload-requested ()
│   sort-changed (field, direction)
└── Slots:
    toolbar-actions (for context-specific buttons)
```

**Data Source Adapters:**
```typescript
interface FileBrowserDataSource {
  listFiles(): Promise<FileEntry[]>;
  getFileContent(path: string): Promise<{ content: string; meta: FileMeta }>;
  saveFileContent(path: string, content: string, expectedVersion?: string): Promise<FileMeta>;
  deleteFile(path: string): Promise<void>;
  uploadFiles(files: File[]): Promise<FileEntry[]>;
  downloadFile(path: string): void;
}
```

Implementations:
- `WorkspaceFileBrowserDataSource` — uses `/api/v1/groves/{id}/workspace/files/...`
- `SharedDirFileBrowserDataSource` — uses `/api/v1/groves/{id}/shared-dirs/{name}/files/...`
- `TemplateFileBrowserDataSource` — uses `/api/v1/templates/{id}/files/...`

### 4.2 Template Expansion Component

```
scion-template-detail (new component, used within grove-settings.ts)
├── Properties:
│   template: Template
│   expanded: boolean
│   capabilities: Capabilities
├── Children:
│   scion-file-browser (with TemplateFileBrowserDataSource)
│   scion-file-editor (opened on edit request)
└── Events:
    template-toggle (templateId)
```

---

## 5. Template Versioning Considerations

### 5.1 Current Model

Templates currently use a two-phase lifecycle:
1. **Create** (status: `pending`) — metadata registered, upload URLs generated.
2. **Finalize** (status: `active`) — files verified, content hash computed.

Once finalized, files are effectively immutable — the API doesn't support modifying individual files in-place. Changes require creating a new template (or clone + modify).

### 5.2 Impact of Inline Editing

Inline editing introduces mutable template files, which conflicts with the current immutable-after-finalize model.

**Approach A: Mutable Active Templates**
- Allow PUT/DELETE on files of active templates.
- Update the content hash after each change.
- Simplest to implement, but breaks the immutability guarantee.
- Risk: runtime brokers may have cached a previous version; the content hash change signals invalidation.

**Approach B: Copy-on-Write Versioning**
- Editing creates a new version of the template (same name/slug, new ID or version number).
- The old version remains available (for rollback, audit).
- More robust but significantly more complex.
- Template references (in agents, grove defaults) would need to track "latest" vs "pinned" versions.

**Approach C: Draft/Publish Workflow**
- Editing puts the template into `draft` status.
- Changes are accumulated in the draft.
- User explicitly "publishes" the draft to make it active.
- Provides a review step but adds UX friction.

**Recommendation:** Start with **Approach A** (mutable active templates) for simplicity. The content hash update mechanism already exists and brokers use it for cache invalidation. Add versioning (Approach B) as a later enhancement when the need for rollback becomes clear.

### 5.3 Locked Template Handling

- Locked templates (`locked: true`) are read-only. The file browser shows files but all mutation actions (edit, delete, upload) are disabled.
- Global templates are typically locked. Grove-scoped templates are not.
- The UI should display a clear lock indicator with explanation text.

---

## 6. Open Questions

1. **Accordion vs. dedicated page** — Should clicking a template expand inline (accordion) or navigate to a dedicated template detail page (`/groves/{id}/templates/{templateId}`)? Accordion keeps context but may feel cramped for templates with many files. A dedicated page offers more space but adds navigation.

2. **Template file tree vs. flat list** — Templates can have nested paths (e.g., `home/.config/settings.json`). Should the browser show a tree view (collapsible directories) or a flat list of full paths? The workspace browser currently uses a flat list. A tree is nicer for deeply nested templates but adds complexity.

3. **Re-use scope** — How far should the file browser abstraction go? Should it also be reused for the agent home directory viewer (if that's ever added)? Over-abstracting early risks building the wrong abstraction.

4. **Template file upload UX** — The current template upload uses signed URLs (two-phase). For the inline browser, should we proxy uploads through the Hub (simpler UX, consistent with workspace upload) or keep the signed-URL flow (better for large files)?

5. **Multi-broker cache invalidation** — When a template file is edited, the content hash changes. How quickly do brokers pick up the change? Is there an explicit invalidation mechanism or is it polling-based? This affects the user's expectation of "I saved, so new agents should use the updated template."

6. **Shared component extraction timing** — Should we extract `scion-file-browser` from `grove-detail.ts` before implementing the template browser, or build the template browser first and extract later? Extracting first is cleaner but delays the template feature.

---

## 7. Implementation Phases

### Phase 1: Template File Browsing (Read-Only)

- [ ] Add `GET /api/v1/templates/{templateId}/files` listing endpoint
- [ ] Add `GET /api/v1/templates/{templateId}/files/{filePath}` content endpoint
- [ ] Add `scion-template-detail` component with accordion expansion in grove settings
- [ ] Display template files in a table (reuse markup patterns from workspace browser, but not yet extracted as shared component)
- [ ] File download via existing signed-URL mechanism
- [ ] Eye icon for file preview (new tab for now)

### Phase 2: Template File Editing

*Depends on: [web-file-editor.md](./web-file-editor.md) Phase 1 (core editor)*

- [ ] Add `PUT /api/v1/templates/{templateId}/files/{filePath}` write endpoint
- [ ] Add `DELETE /api/v1/templates/{templateId}/files/{filePath}` delete endpoint
- [ ] Integrate `scion-file-editor` into template detail view
- [ ] Pencil icon on template files (gated on capabilities + not locked)
- [ ] Lock indicator for locked/global templates
- [ ] Content hash recomputation on save

### Phase 3: Shared Component Extraction

*Depends on: Phase 2 + [web-file-editor.md](./web-file-editor.md) Phase 1 workspace integration*

- [ ] Extract `scion-file-browser` shared component from `grove-detail.ts`
- [ ] Implement `FileBrowserDataSource` adapter pattern
- [ ] Refactor workspace browser to use shared component
- [ ] Refactor template browser to use shared component
- [ ] Refactor shared-dir browser to use shared component

### Phase 4: Polish

- [ ] Template file upload (add files to existing template)
- [ ] Markdown preview for template `.md` files via eye icon
- [ ] Tree view for nested template file structures (stretch)
- [ ] Template ZIP download (stretch)

---

## 8. Dependencies

- **Web File Editor** ([web-file-editor.md](./web-file-editor.md)) — the editor component is a prerequisite for Phase 2.
- **Template API** — existing CRUD and download endpoints. New file-level endpoints needed.
- **Storage layer** — `pkg/storage/` must support reading individual files by path (currently supports full-prefix operations).
- **Shoelace icons** — `chevron-right`, `chevron-down` (for accordion), `lock` (for locked templates). Check if already in `USED_ICONS`.
