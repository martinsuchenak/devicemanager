import Alpine from 'alpinejs';
import { api } from './api.js';
import { modalConfig, viewModalConfig } from './modal.js';

Alpine.data('datacenterManager', () => ({
    // Reusable modal configurations
    editModal: modalConfig('lg'),
    viewModal: viewModalConfig('lg'),

    get datacenters() { return Alpine.store('appData').datacenters; },
    get loading() { return Alpine.store('appData').loadingDatacenters; },
    saving: false,
    // For backward compatibility with existing HTML
    get showModal() { return this.editModal.show; },
    set showModal(value) { this.editModal.show = value; },
    get showViewModal() { return this.viewModal.show; },
    set showViewModal(value) { this.viewModal.show = value; },
    modalTitle: 'Add Datacenter',
    currentDatacenter: {},
    form: { id: '', name: '', location: '', description: '' },

    init() {
        Alpine.store('appData').loadDatacenters();
        // Listen for refresh events
        window.addEventListener('refresh-datacenters', () => Alpine.store('appData').loadDatacenters(true));
    },

    openAddModal() {
        this.modalTitle = 'Add Datacenter';
        this.resetForm();
        this.editModal.open();
    },

    closeModal() {
        this.editModal.close();
        this.resetForm();
    },

    resetForm() {
        this.form = { id: '', name: '', location: '', description: '' };
    },

    async saveDatacenter() {
        this.saving = true;
        try {
            const payload = {
                name: this.form.name,
                location: this.form.location || '',
                description: this.form.description || ''
            };

            if (this.form.id) {
                await api.put(`/api/datacenters/${this.form.id}`, payload);
                Alpine.store('toast').notify('Datacenter updated successfully', 'success');
            } else {
                await api.post('/api/datacenters', payload);
                Alpine.store('toast').notify('Datacenter created successfully', 'success');
            }

            this.closeModal();
            Alpine.store('appData').loadDatacenters(true);
            // Dispatch event for other components
            window.dispatchEvent(new CustomEvent('refresh-datacenters'));
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async viewDatacenter(id) {
        try {
            const datacenter = await api.get(`/api/datacenters/${id}`);
            this.viewModal.openWithItem(datacenter);
        } catch (error) {
            Alpine.store('toast').notify('Failed to load datacenter', 'error');
        }
    },

    closeViewModal() {
        this.viewModal.close();
    },

    editCurrentDatacenter() {
        const dc = this.viewModal.currentItem;
        this.prepareEditForm(dc);
        this.viewModal.close();
        this.editModal.open();
    },

    async editDatacenter(id) {
        try {
            const datacenter = await api.get(`/api/datacenters/${id}`);
            this.prepareEditForm(datacenter);
            this.editModal.open();
        } catch (error) {
            Alpine.store('toast').notify('Failed to load datacenter', 'error');
        }
    },

    prepareEditForm(datacenter) {
        this.modalTitle = 'Edit Datacenter';
        this.form = {
            id: datacenter.id || '',
            name: datacenter.name || '',
            location: datacenter.location || '',
            description: datacenter.description || ''
        };
    },

    async deleteDatacenter(id) {
        // Check for associated devices
        let deviceCount = 0;
        try {
            const devices = await api.get(`/api/datacenters/${id}/devices`);
            deviceCount = devices.length;
        } catch (e) { /* ignore error */ }

        const message = deviceCount > 0
            ? `Are you sure you want to delete this datacenter? ${deviceCount} devices will lose their association.`
            : 'Are you sure you want to delete this datacenter?';

        if (!confirm(message)) return;

        try {
            await api.delete(`/api/datacenters/${id}`);
            Alpine.store('toast').notify('Datacenter deleted successfully', 'success');
            Alpine.store('appData').loadDatacenters(true);
            window.dispatchEvent(new CustomEvent('refresh-datacenters'));
            if (this.viewModal.show && this.viewModal.currentItem?.id === id) {
                this.viewModal.close();
            }
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete datacenter', 'error');
        }
    },

    deleteCurrentDatacenter() {
        this.deleteDatacenter(this.viewModal.currentItem?.id);
    }
}));
