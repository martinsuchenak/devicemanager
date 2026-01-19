# Rackd UI Guidelines

This document establishes the UI/UX standards and conventions for the Rackd application. All UI changes should follow these guidelines to ensure consistency across the application.

## Design System

### Color Palette

#### Button Colors
| Purpose | Light Mode | Dark Mode | Usage |
|---------|------------|------------|-------|
| Primary actions | `bg-blue-600 hover:bg-blue-700` | `dark:bg-blue-500 dark:hover:bg-blue-600` | Save, Add, Promote, Start Scan, etc. |
| Destructive actions | `bg-red-600 hover:bg-red-700` | `dark:bg-red-500 dark:hover:bg-red-600` | Delete, Remove |
| Special call-to-action | `bg-green-600 hover:bg-green-700` | `dark:bg-green-500 dark:hover:bg-green-600` | Promote to Device (in view modal) |
| Secondary actions | `bg-gray-200 hover:bg-gray-300 text-gray-700` | `dark:bg-gray-700 dark:text-gray-200` | Cancel, Close, Clear Filters |

#### Status Badges
| Status | Classes |
|--------|---------|
| Online | `bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200` |
| Offline | `bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200` |
| Unknown | `bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300` |

#### Confidence Levels
| Range | Classes |
|-------|---------|
| 80-100% | `bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200` |
| 50-79% | `bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200` |
| 0-49% | `bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200` |

### Typography
- **Headings:** `text-lg font-semibold` (modal titles), `text-2xl font-bold` (page titles)
- **Labels:** `text-sm font-medium text-gray-700 dark:text-gray-300`
- **Body text:** `text-gray-900 dark:text-gray-100`
- **Muted text:** `text-gray-500 dark:text-gray-400`

### Spacing
- **Modal headers:** `p-6` or `px-6 py-4`
- **Modal bodies:** `p-6` or `px-6 py-4 max-h-[70vh] overflow-y-auto`
- **Form spacing:** `space-y-4` (between form fields)
- **Button gaps:** `gap-3` (between buttons in a row)

## Modal Patterns

### Standard Modal Structure

```html
<div x-show="showModal" class="fixed inset-0 z-50 overflow-y-auto" role="dialog" aria-modal="true"
    x-trap.noscroll="showModal">
    <div class="flex min-h-screen items-center justify-center p-4">
        <div x-show="showModal" :class="editModal.backdropClass" @click="closeModal()"></div>
        <div :class="editModal.modalClass">
            <!-- Header -->
            <div class="flex items-center justify-between p-6 border-b dark:border-gray-700">
                <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100" x-text="modalTitle"></h3>
                <button type="button" @click="closeModal()" class="text-gray-400 hover:text-gray-500">
                    <!-- close icon -->
                </button>
            </div>

            <!-- Body -->
            <div class="px-6 py-4 max-h-[70vh] overflow-y-auto">
                <form @submit.prevent="saveItem" class="space-y-4">
                    <!-- form fields -->
                </form>
            </div>

            <!-- Footer -->
            <div :class="editModal.footerClass">
                <button @click="saveItem" :disabled="saving" class="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50">Save</button>
                <button @click="closeModal()" class="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300">Cancel</button>
            </div>
        </div>
    </div>
</div>
```

### Modal Configurations

Use the reusable modal configurations from `webui/src/modal.js`:

| Config | Size | Use Case |
|--------|------|----------|
| `modalConfig('sm')` | max-w-sm | Small confirmations |
| `modalConfig('md')` | max-w-md | Simple forms (scan modal) |
| `modalConfig('lg')` | max-w-lg | Standard forms (network, datacenter) |
| `modalConfig('xl')` | max-w-xl | Larger forms |
| `modalConfig('2xl')` | max-w-2xl | Complex forms (promote device) |
| `modalConfig('3xl')` | max-w-3xl | Very large forms (device edit with addresses) |
| `viewModalConfig(size)` | (any size) | View-only modals with `currentItem` support |

### Modal Implementation in Components

```javascript
import { modalConfig, viewModalConfig } from './modal.js';

Alpine.data('myManager', () => ({
    // Modal configurations
    editModal: modalConfig('lg'),
    viewModal: viewModalConfig('lg'),

    // For backward compatibility with HTML
    get showModal() { return this.editModal.show; },
    set showModal(value) { this.editModal.show = value; },
    get showViewModal() { return this.viewModal.show; },
    set showViewModal(value) { this.viewModal.show = value; },

    openAddModal() {
        this.resetForm();
        this.editModal.open();
    },

    closeModal() {
        this.editModal.close();
        this.resetForm();
    },

    async viewItem(id) {
        const item = await api.get(`/api/items/${id}`);
        this.viewModal.openWithItem(item);
    }
}));
```

### Modal Styling Classes

The modal config provides these computed classes:

- **backdropClass:** `bg-gray-900/20 backdrop-blur-sm transition-opacity dark:bg-gray-900/50`
- **modalClass:** `relative z-10 bg-white rounded-lg shadow-xl max-w-{size} w-full dark:bg-gray-800`
- **footerClass:** `border-t border-gray-200 px-6 py-4 bg-gray-50 dark:border-gray-700 dark:bg-gray-900 flex gap-3`

## Form Patterns

### Standard Form Layout

```html
<form @submit.prevent="saveItem" class="space-y-4">
    <!-- Required field -->
    <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Field Name *</label>
        <input type="text" x-model="form.field" required
            class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none bg-white text-gray-900 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100">
    </div>

    <!-- Optional field -->
    <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Optional Field</label>
        <textarea x-model="form.optional" rows="2"
            class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"></textarea>
    </div>

    <!-- Select dropdown -->
    <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Dropdown</label>
        <select x-model="form.choice" required
            class="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100">
            <option value="">Select an option</option>
            <option value="option1">Option 1</option>
        </select>
    </div>
</form>
```

### Input Styles

All text inputs and textareas should use:
```
mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg
focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none
bg-white text-gray-900
dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100
```

### Required Fields

- Mark required fields with `*` in the label
- Add `required` attribute to the input element
- Use `placeholder` attribute for help text (e.g., "e.g. rackd-server-01")

## Button Patterns

### Action Buttons (in forms/footers)

```html
<!-- Primary action -->
<button @click="saveItem" :disabled="saving"
    class="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 dark:bg-blue-500 dark:hover:bg-blue-600">
    Save
</button>

<!-- Secondary action -->
<button @click="cancel()"
    class="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200">
    Cancel
</button>

<!-- Destructive action -->
<button @click="deleteItem()"
    class="flex-1 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600">
    Delete
</button>
```

### Icon Buttons

```html
<button @click="action()"
    class="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600">
    <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <!-- icon path -->
    </svg>
    Button Text
</button>
```

### Link Actions (in tables)

```html
<button @click="viewItem(item.id)" class="text-blue-600 hover:text-blue-900 dark:text-blue-400 mr-2">View</button>
<button @click="editItem(item.id)" class="text-blue-600 hover:text-blue-900 dark:text-blue-400 mr-2">Edit</button>
<button @click="deleteItem(item.id)" class="text-red-600 hover:text-red-900 dark:text-red-400">Delete</button>
```

## Table Patterns

### Standard Table

```html
<div class="bg-white shadow rounded-lg overflow-hidden dark:bg-gray-800">
    <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead class="bg-gray-50 dark:bg-gray-900">
                <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider dark:text-gray-400">Column</th>
                </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200 dark:bg-gray-800 dark:divide-gray-700">
                <template x-for="item in items" :key="item.id">
                    <tr class="hover:bg-gray-50 dark:hover:bg-gray-700">
                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">Content</td>
                    </tr>
                </template>
            </tbody>
        </table>
    </div>
</div>
```

## Loading States

### Page Loading

```html
<div x-show="loading" class="text-center py-12">
    <div class="inline-block animate-spin rounded-full h-8 w-8 border-4 border-blue-600 border-t-transparent"></div>
    <p class="mt-2 text-gray-600 dark:text-gray-400">Loading items...</p>
</div>
```

### Button Loading

```html
<button @click="saveItem" :disabled="saving"
    class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50">
    <span x-show="!saving">Save</span>
    <span x-show="saving">Saving...</span>
</button>
```

## Empty States

```html
<div x-show="items.length === 0" class="text-center py-12">
    <p class="text-gray-500 dark:text-gray-400">No items found.</p>
    <p class="text-gray-400 dark:text-gray-500 mt-2">Create your first item to get started.</p>
</div>
```

## Toast Notifications

Toast notifications are available via `Alpine.store('toast')`:

```javascript
// Success toast
Alpine.store('toast').notify('Item saved successfully', 'success');

// Error toast
Alpine.store('toast').notify('Failed to save item', 'error');

// Info toast
Alpine.store('toast').notify('Processing...', 'info');
```

## Dark Mode

All components must support dark mode. Use these patterns:

- Backgrounds: `dark:bg-gray-800`, `dark:bg-gray-700`, `dark:bg-gray-900`
- Text: `dark:text-gray-100`, `dark:text-gray-200`, `dark:text-gray-300`, `dark:text-gray-400`
- Borders: `dark:border-gray-600`, `dark:border-gray-700`
- Buttons: Always specify the dark mode variant (e.g., `dark:bg-blue-500 dark:hover:bg-blue-600`)

## Accessibility

- All modals must have `role="dialog"` and `aria-modal="true"`
- All modals must have `x-trap.noscroll` for focus trapping
- Use semantic HTML (`<label>`, `<button>`, `<input>`)
- Include `aria-label` for icon-only buttons
- Maintain proper tab order with logical focus flow
- Use `sr-only` class for screen reader only text (defined in Tailwind)

## Consistency Rules

1. **Always use the modal config** from `modal.js` - don't hardcode modal classes
2. **Always use `x-trap.noscroll`** on modals for focus trapping
3. **Always add `aria-modal="true"` to modals
4. **Always use optional chaining (`?.`)** for accessing null-possible properties (e.g., `viewModal.currentItem?.name`)
5. **Always include dark mode variants** for all colored elements
6. **Always use flex-1 for buttons in footers** to ensure equal width
7. **Always disable buttons during async operations** (`:disabled="saving"`)
8. **Never use `style="display: none;"`** - let Alpine's `x-show` handle visibility

## Component Best Practices

1. **Use Alpine.store for shared state** - `appData`, `toast`, `router`, `enterprise`
2. **Keep components focused** - One manager per page/section
3. **Use computed properties** with `get` for derived state
4. **Emit custom events** for cross-component communication (e.g., `refresh-devices`)
5. **Initialize data in `init()`** - Load dependencies when component mounts
6. **Use `x-show` for conditional rendering** - It handles display:none properly
7. **Use `x-for` for lists** - Always provide a unique `:key`
