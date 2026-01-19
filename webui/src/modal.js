// Reusable modal configuration for Alpine.js components
// Provides consistent modal styling and behavior across the app

/**
 * Creates a modal configuration object with standard backdrop and modal styling
 * @param {string} size - Modal size: 'sm', 'md', 'lg', 'xl', '2xl', '3xl', 'full'
 * @returns {Object} Modal configuration with backdropClass, modalClass, and standard methods
 */
export function modalConfig(size = 'md') {
    const sizeClasses = {
        sm: 'max-w-sm',
        md: 'max-w-md',
        lg: 'max-w-lg',
        xl: 'max-w-xl',
        '2xl': 'max-w-2xl',
        '3xl': 'max-w-3xl',
        full: 'max-w-full'
    };

    return {
        // Modal state
        show: false,

        // CSS classes for consistent styling
        get backdropClass() {
            return 'fixed inset-0 bg-gray-900/20 backdrop-blur-sm transition-opacity dark:bg-gray-900/50';
        },

        get modalClass() {
            const sizeClass = sizeClasses[size] || sizeClasses.md;
            return `relative z-10 bg-white rounded-lg shadow-xl ${sizeClass} w-full dark:bg-gray-800`;
        },

        get footerClass() {
            return 'border-t border-gray-200 px-6 py-4 bg-gray-50 dark:border-gray-700 dark:bg-gray-900 flex gap-3';
        },

        // Standard modal methods
        open() {
            this.show = true;
        },

        close() {
            this.show = false;
        },

        toggle() {
            this.show = !this.show;
        }
    };
}

/**
 * Creates a specialized modal configuration for modals that need to track a current item
 * @param {string} size - Modal size
 * @returns {Object} Modal configuration with currentItem support
 */
export function viewModalConfig(size = 'lg') {
    return {
        ...modalConfig(size),
        currentItem: null,

        openWithItem(item) {
            this.currentItem = item;
            this.show = true;
        },

        close() {
            this.show = false;
            this.currentItem = null;
        }
    };
}
