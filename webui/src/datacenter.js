import Alpine from 'alpinejs';
import { api } from './api.js';

Alpine.data('datacenterManager', () => ({
    get datacenters() { return Alpine.store('appData').datacenters; },
    get loading() { return Alpine.store('appData').loadingDatacenters; },
    saving: false,
    showModal: false,
    showViewModal: false,
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
        this.showModal = true;
    },

    closeModal() {
        this.showModal = false;
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
            this.currentDatacenter = await api.get(`/api/datacenters/${id}`);
            this.showViewModal = true;
        } catch (error) {
            Alpine.store('toast').notify('Failed to load datacenter', 'error');
        }
    },

    closeViewModal() {
        this.showViewModal = false;
        this.currentDatacenter = {};
    },

    editCurrentDatacenter() {
        const dc = this.currentDatacenter;
        this.prepareEditForm(dc);
        this.closeViewModal();
        this.showModal = true;
    },

    async editDatacenter(id) {
        try {
            const datacenter = await api.get(`/api/datacenters/${id}`);
            this.prepareEditForm(datacenter);
            this.showModal = true;
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
            if (this.showViewModal && this.currentDatacenter.id === id) {
                this.closeViewModal();
            }
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete datacenter', 'error');
        }
    },

    deleteCurrentDatacenter() {
        this.deleteDatacenter(this.currentDatacenter.id);
    }
}));