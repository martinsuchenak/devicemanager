import Alpine from 'alpinejs';

// Global Toast Store
Alpine.store('toast', {
    show: false,
    message: '',
    type: 'info',
    notify(message, type = 'info') {
        this.message = message;
        this.type = type;
        this.show = true;
        setTimeout(() => { this.show = false; }, 3000);
    }
});