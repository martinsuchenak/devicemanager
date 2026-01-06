import Alpine from 'alpinejs';
import { api } from './api.js';

Alpine.data('networkManager', () => ({
    get networks() {
        // Enrich networks with datacenter names on the fly
        return Alpine.store('appData').networks.map(n => ({
            ...n,
            datacenter_name: Alpine.store('appData').getDatacenterName(n.datacenter_id)
        }));
    },
    get datacenters() { return Alpine.store('appData').datacenters; },
    get loading() { return Alpine.store('appData').loadingNetworks; },
    saving: false,
    showModal: false,
    showViewModal: false,
    modalTitle: 'Add Network',
    currentNetwork: {},
    form: { id: '', name: '', subnet: '', datacenter_id: '', description: '' },

    init() {
        Alpine.store('appData').loadNetworks();
        Alpine.store('appData').loadDatacenters();
        window.addEventListener('refresh-networks', () => Alpine.store('appData').loadNetworks(true));
        window.addEventListener('refresh-datacenters', () => Alpine.store('appData').loadDatacenters(true));
    },

    // Check if there's only one datacenter
    get hasSingleDatacenter() {
        return this.datacenters.length === 1;
    },

    // Get the single datacenter ID if there's only one
    get singleDatacenterId() {
        return this.hasSingleDatacenter ? this.datacenters[0].id : null;
    },

    openAddModal() {
        this.modalTitle = 'Add Network';
        this.resetForm();
        this.showModal = true;
    },

    closeModal() {
        this.showModal = false;
        this.resetForm();
    },

    resetForm() {
        this.form = { id: '', name: '', subnet: '', datacenter_id: '', description: '' };
        // Auto-select the single datacenter if there's only one
        if (this.hasSingleDatacenter) {
            this.form.datacenter_id = this.singleDatacenterId;
        }
    },

    async saveNetwork() {
        this.saving = true;
        try {
            const payload = {
                name: this.form.name,
                subnet: this.form.subnet,
                datacenter_id: this.form.datacenter_id,
                description: this.form.description || ''
            };

            if (this.form.id) {
                await api.put(`/api/networks/${this.form.id}`, payload);
                Alpine.store('toast').notify('Network updated successfully', 'success');
            } else {
                await api.post('/api/networks', payload);
                Alpine.store('toast').notify('Network created successfully', 'success');
            }

            this.closeModal();
            Alpine.store('appData').loadNetworks(true);
            window.dispatchEvent(new CustomEvent('refresh-networks'));
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async viewNetwork(id) {
        try {
            const network = await api.get(`/api/networks/${id}`);
            network.datacenter_name = this.datacenters.find(dc => dc.id === network.datacenter_id)?.name || null;
            this.currentNetwork = network;
            this.showViewModal = true;
        } catch (error) {
            Alpine.store('toast').notify('Failed to load network', 'error');
        }
    },

    closeViewModal() {
        this.showViewModal = false;
        this.currentNetwork = {};
    },

    editCurrentNetwork() {
        const network = this.currentNetwork;
        this.prepareEditForm(network);
        this.closeViewModal();
        this.showModal = true;
    },

    async editNetwork(id) {
        try {
            const network = await api.get(`/api/networks/${id}`);
            this.prepareEditForm(network);
            this.showModal = true;
        } catch (error) {
            Alpine.store('toast').notify('Failed to load network', 'error');
        }
    },

    prepareEditForm(network) {
        this.modalTitle = 'Edit Network';
        this.form = {
            id: network.id || '',
            name: network.name || '',
            subnet: network.subnet || '',
            datacenter_id: network.datacenter_id || '',
            description: network.description || ''
        };
    },

    async deleteNetwork(id) {
        let deviceCount = 0;
        try {
            const devices = await api.get(`/api/networks/${id}/devices`);
            deviceCount = devices.length;
        } catch (e) { /* ignore */ }

        const message = deviceCount > 0
            ? `Are you sure you want to delete this network? ${deviceCount} devices will lose their network association.`
            : 'Are you sure you want to delete this network?';

        if (!confirm(message)) return;

        try {
            await api.delete(`/api/networks/${id}`);
            Alpine.store('toast').notify('Network deleted successfully', 'success');
            Alpine.store('appData').loadNetworks(true);
            window.dispatchEvent(new CustomEvent('refresh-networks'));
            if (this.showViewModal && this.currentNetwork.id === id) {
                this.closeViewModal();
            }
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete network', 'error');
        }
    },

    deleteCurrentNetwork() {
        this.deleteNetwork(this.currentNetwork.id);
    }
}));