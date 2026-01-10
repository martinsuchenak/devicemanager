import Alpine from 'alpinejs';
import focus from '@alpinejs/focus';
import { api } from './api.js';
import './toast.js';
import './datacenter.js';
import './network.js';
import './device.js';
import './discovery.js';

Alpine.plugin(focus);

Alpine.store('appData', {
    datacenters: [],
    networks: [],
    loadingDatacenters: false,
    loadingNetworks: false,
    _datacentersPromise: null,
    _networksPromise: null,

    async loadDatacenters(force = false) {
        if (this._datacentersPromise && !force) return this._datacentersPromise;
        this.loadingDatacenters = true;
        this._datacentersPromise = (async () => {
            try {
                const data = await api.get('/api/datacenters');
                this.datacenters = Array.isArray(data) ? data : [];
            } catch (error) {
                Alpine.store('toast').notify('Failed to load datacenters', 'error');
                this.datacenters = [];
            } finally {
                this.loadingDatacenters = false;
            }
            return this.datacenters;
        })();
        return this._datacentersPromise;
    },

    async loadNetworks(force = false) {
        if (this._networksPromise && !force) return this._networksPromise;
        this.loadingNetworks = true;
        this._networksPromise = (async () => {
            try {
                const data = await api.get('/api/networks');
                this.networks = Array.isArray(data) ? data : [];
            } catch (error) {
                Alpine.store('toast').notify('Failed to load networks', 'error');
                this.networks = [];
            } finally {
                this.loadingNetworks = false;
            }
            return this.networks;
        })();
        return this._networksPromise;
    },

    getDatacenterName(id) {
        return this.datacenters.find(dc => dc.id === id)?.name || null;
    },

    getNetworkName(id) {
        return this.networks.find(n => n.id === id)?.name || null;
    }
});

Alpine.data('poolManager', () => ({
    pools: [],
    loading: false,
    networkId: null,
    showModal: false,
    modalTitle: 'Add Network Pool',
    form: { id: '', name: '', start_ip: '', end_ip: '', description: '' },

    init() {
        // Listen for events to open manager for a specific network
        window.addEventListener('manage-pools', (e) => {
            this.networkId = e.detail.networkId;
            this.showPoolForm = false;
            this.loadPools();
            this.showModal = true;
        });
    },

    async loadPools() {
        if (!this.networkId) return;
        this.loading = true;
        try {
            const data = await api.get(`/api/networks/${this.networkId}/pools`);
            this.pools = Array.isArray(data) ? data : [];
        } catch (error) {
            Alpine.store('toast').notify('Failed to load pools', 'error');
        } finally {
            this.loading = false;
        }
    },

    editingPool: null,
    showPoolForm: false,

    startEdit(pool) {
        this.editingPool = pool;
        this.form = { ...pool };
        this.showPoolForm = true;
        this.modalTitle = 'Edit Network Pool';
    },

    startAdd() {
        this.editingPool = null;
        this.form = { id: '', name: '', start_ip: '', end_ip: '', description: '' };
        this.showPoolForm = true;
        this.modalTitle = 'Add Network Pool';
    },

    cancelEdit() {
        this.showPoolForm = false;
        this.loadPools();
    },

    async savePool() {
        try {
            const payload = { ...this.form, network_id: this.networkId };
            if (this.editingPool) {
                await api.put(`/api/pools/${this.form.id}`, payload);
                Alpine.store('toast').notify('Pool updated', 'success');
            } else {
                await api.post(`/api/networks/${this.networkId}/pools`, payload);
                Alpine.store('toast').notify('Pool created', 'success');
            }
            this.showPoolForm = false;
            this.loadPools();
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        }
    },

    async deletePool(id) {
        if (!confirm('Are you sure?')) return;
        try {
            await api.delete(`/api/pools/${id}`);
            Alpine.store('toast').notify('Pool deleted', 'success');
            this.loadPools();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete pool', 'error');
        }
    }
}));

Alpine.start();