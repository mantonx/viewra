# ADR 033: Settings UX/UI Improvements

## Status

Accepted (Implemented December 3, 2025)

## Date

December 2, 2025

## Context

ADR 032 established the settings infrastructure with environment variable awareness, source tracking, and read-only display values. The backend implementation is complete, but the frontend UX has several issues that make the settings page difficult to use:

### Current UX Problems

1. **Per-setting Save/Cancel buttons**: Each setting has its own Save/Cancel buttons that appear when edited. This is tedious when changing multiple settings and creates visual clutter.

2. **Edit-on-change model**: Any change immediately marks the setting as "edited" with inline Save/Cancel appearing. This creates UI churn and makes it hard to make multiple changes at once.

3. **Source badges floating right**: The source indicators (Default, Saved, Env Var, Detected) are positioned far from the setting label, making it hard to associate them with the correct setting.

4. **Read-only settings mixed with editable**: Same visual treatment for both types makes it hard to scan what you can actually change.

5. **Disconnected action flow**: The "unsaved changes" banner at top is disconnected from the actual save actions at each individual setting.

6. **No restart warnings**: Some settings require server restart but there's no clear indication when editing them.

7. **No form validation feedback**: Integer settings with min/max constraints don't show validation errors.

8. **Description placement inconsistent**: Some descriptions are in helper text, some are tooltips, making the UI feel inconsistent.

### User Mental Model Issues

- Users expect a traditional form: make changes → review → save all
- Current per-field save feels like a spreadsheet, not a settings page
- Hard to "undo all changes" and start over
- No confirmation of what will change before saving

## Decision

Redesign the settings UX with a form-based save model and clearer visual hierarchy.

### UX Principles

1. **Category-level save**: One "Save Changes" button per category card
2. **Clear edit states**: Visual distinction when viewing vs editing
3. **Grouped by editability**: Separate detected/locked settings from configurable ones
4. **Inline source indicators**: Show source next to label, not floating right
5. **Restart warnings**: Clear callout for settings that need restart
6. **Form validation**: Show validation errors before save attempt

### Layout Redesign

```text
┌─────────────────────────────────────────────────────────────────────────┐
│  System Settings                                                        │
│  Configure server-wide settings that affect all users                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  SYSTEM HARDWARE                                    [Detected]  │    │
│  │  ─────────────────────────────────────────────────────────────  │    │
│  │                                                                  │    │
│  │  CPU          AMD Ryzen 9 5900X                                 │    │
│  │               12 cores / 24 threads                              │    │
│  │                                                                  │    │
│  │  Memory       32 GB                                              │    │
│  │                                                                  │    │
│  │  GPU          NVIDIA GeForce RTX 3080                           │    │
│  │               nvenc · vaapi                                      │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  TRANSCODING                                                     │    │
│  │  ─────────────────────────────────────────────────────────────  │    │
│  │                                                                  │    │
│  │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │    │
│  │  READ-ONLY VALUES                                                │    │
│  │  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │    │
│  │                                                                  │    │
│  │  Detected Hardware Acceleration     NVIDIA NVENC    [Detected]  │    │
│  │  Auto-detected based on available GPU                            │    │
│  │                                                                  │    │
│  │  ─────────────────────────────────────────────────────────────  │    │
│  │  CONFIGURABLE                                                    │    │
│  │  ─────────────────────────────────────────────────────────────  │    │
│  │                                                                  │    │
│  │  Hardware Acceleration Override                         [Default]│    │
│  │  ┌────────────────────────────────────────────────────────────┐ │    │
│  │  │ Auto (use detected)                                      ▼ │ │    │
│  │  └────────────────────────────────────────────────────────────┘ │    │
│  │  Override auto-detected hardware acceleration                    │    │
│  │                                                                  │    │
│  │  Default Quality                                        [Saved] │    │
│  │  ┌────────────────────────────────────────────────────────────┐ │    │
│  │  │ 720p (HD)                                               ▼ │ │    │
│  │  └────────────────────────────────────────────────────────────┘ │    │
│  │  Default video transcoding quality                               │    │
│  │                                                                  │    │
│  │  Transcode Workers                    ⚠️ Requires Restart [Env] │    │
│  │  ┌────────────────────────────────────────────────────────────┐ │    │
│  │  │ 8                                                          │ │    │
│  │  └────────────────────────────────────────────────────────────┘ │    │
│  │  Number of concurrent transcode jobs                             │    │
│  │  🔒 Controlled by TRANSCODE_WORKERS environment variable        │    │
│  │                                                                  │    │
│  │  HDR Tone Mapping                                       [Default]│    │
│  │  [✓] Enable HDR to SDR conversion                                │    │
│  │  Automatically convert HDR content for SDR displays             │    │
│  │                                                                  │    │
│  │  ─────────────────────────────────────────────────────────────  │    │
│  │                               [ Discard Changes ] [ Save ]      │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Key UX Changes

#### 1. Category-Level Save Model

Instead of per-field save buttons:
- Each category card has a footer with "Discard Changes" and "Save" buttons
- Buttons only appear when there are unsaved changes in that category
- Save button shows count of changes: "Save (3 changes)"
- Saving one category doesn't affect pending changes in others

```tsx
// Category card footer when there are changes
<CardFooter className="flex justify-end gap-2 border-t">
  <Button variant="ghost" onClick={discardCategoryChanges}>
    Discard Changes
  </Button>
  <Button onClick={saveCategoryChanges} isLoading={saving}>
    Save {changedCount > 1 ? `(${changedCount} changes)` : ''}
  </Button>
</CardFooter>
```

#### 2. Visual Grouping by Editability

Within each category, separate:
- **Read-only section**: Detected values, env-var locked values (gray background)
- **Configurable section**: Editable settings (white/dark background)

```tsx
<div className="space-y-6">
  {hasReadOnlySettings && (
    <div className="bg-neutral-50 dark:bg-neutral-900/50 rounded-lg p-4">
      <div className="text-xs font-medium text-neutral-500 uppercase tracking-wider mb-3">
        Read-only Values
      </div>
      {readOnlySettings.map(setting => <ReadOnlySetting key={setting.key} {...setting} />)}
    </div>
  )}

  <div className="space-y-4">
    {editableSettings.map(setting => <EditableSetting key={setting.key} {...setting} />)}
  </div>
</div>
```

#### 3. Inline Source Indicators

Move source badge to be inline with the label:

```text
Before:
  Default Quality                                               [Saved]

After:
  Default Quality [Saved]
```

For locked settings, show lock icon and reason inline:

```text
  Transcode Workers [🔒 TRANSCODE_WORKERS]
```

#### 4. Restart Warning Badges

Settings with `restartable: true` show a warning badge:

```tsx
{setting.restartable && (
  <span className="inline-flex items-center gap-1 text-xs text-amber-600">
    <AlertTriangle className="w-3 h-3" />
    Requires restart
  </span>
)}
```

#### 5. Pending Changes Indicator

Show changed fields with a subtle highlight:

```tsx
<div className={cn(
  'py-4 border-l-2 pl-4 -ml-4',
  isChanged ? 'border-amber-500 bg-amber-50/50 dark:bg-amber-950/20' : 'border-transparent'
)}>
  {/* setting content */}
</div>
```

#### 6. Validation Feedback

Show validation errors inline before save:

```tsx
<Input
  type="number"
  value={value}
  onChange={handleChange}
  min={validation?.min}
  max={validation?.max}
  error={validationError}
/>
{validationError && (
  <p className="text-sm text-red-600">{validationError}</p>
)}
```

### Component Structure

```text
SystemSettings
├── SystemHardwareCard (read-only, always visible)
│   ├── CPU info
│   ├── Memory info
│   └── GPU info with capabilities
│
├── CategoryCard (one per category)
│   ├── CategoryHeader
│   │   ├── Icon + Title
│   │   └── Change count badge (if any)
│   │
│   ├── ReadOnlySection (if has read-only settings)
│   │   └── ReadOnlySetting[]
│   │
│   ├── EditableSection
│   │   └── EditableSetting[]
│   │       ├── Label + SourceBadge + RestartBadge
│   │       ├── Input/Select/Toggle
│   │       └── Description + ValidationError
│   │
│   └── CategoryFooter (if has changes)
│       ├── Discard button
│       └── Save button
│
└── PageFooter
    └── Reload Settings button
```

### State Management

```tsx
// Per-category change tracking
const [categoryChanges, setCategoryChanges] = useState<Record<string, Record<string, unknown>>>({
  transcoding: {},
  scanning: {},
  server: {},
})

// Check if category has changes
const hasCategoryChanges = (category: string) =>
  Object.keys(categoryChanges[category] || {}).length > 0

// Save single category
const saveCategoryChanges = async (category: string) => {
  const changes = categoryChanges[category]
  for (const [key, value] of Object.entries(changes)) {
    await putApiSettingsSystemKey(key, { value })
  }
  // Clear category changes after save
  setCategoryChanges(prev => ({ ...prev, [category]: {} }))
  refetchSettings()
}

// Discard single category
const discardCategoryChanges = (category: string) => {
  setCategoryChanges(prev => ({ ...prev, [category]: {} }))
}
```

### Visual Design Tokens

#### Source Badges

| Source | Badge Color | Icon |
|--------|-------------|------|
| Default | Gray | — |
| Saved | Green | Check |
| Env Var | Amber + Lock | Lock |
| Detected | Blue | Cpu |

#### State Colors

| State | Background | Border |
|-------|------------|--------|
| Read-only section | neutral-50/900 | — |
| Changed field | amber-50/950 | amber-500 left border |
| Validation error | — | red-500 |
| Locked input | neutral-100/800 | — |

### Accessibility

- All form fields have proper labels and aria attributes
- Source badges have tooltip explanations
- Restart warnings announced to screen readers
- Focus management when save completes
- Keyboard navigation through all settings

## Consequences

### Positive

- Cleaner, less cluttered interface
- Easier to make multiple changes at once
- Clear visual distinction between read-only and editable
- Obvious which settings need restart
- Validation before save reduces errors
- Familiar form-based UX pattern

### Negative

- Larger diff when saving multiple settings
- Can't save individual settings without affecting others in category
- Slightly more complex state management

### Neutral

- Different UX pattern from current implementation
- Requires frontend refactor

## Implementation Plan

### Phase 1: State Management Refactor

1. Refactor from per-setting to per-category change tracking
2. Update save/discard logic for category-level operations
3. Add validation logic with error state

### Phase 2: Layout Restructure

1. Create ReadOnlySection and EditableSection components
2. Move source badges inline with labels
3. Add restart warning badges
4. Create CategoryFooter with conditional save buttons

### Phase 3: Visual Polish

1. Add pending change highlights
2. Improve source badge designs
3. Add loading states for save operations
4. Animate footer appearance

### Phase 4: Accessibility & Testing

1. Add aria attributes and focus management
2. Test with keyboard navigation
3. Verify screen reader announcements

**Estimated Effort**: 1-2 days

## References

- Depends on: [ADR 032 - Settings Infrastructure v2](032-settings-infrastructure-v2.md)
- Related: [ADR 031 - Design System Improvements](031-design-system-improvements.md)
