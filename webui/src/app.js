import Alpine from 'alpinejs';
import focus from '@alpinejs/focus';

Alpine.plugin(focus);

// Shared API helper
const api = {
    async request(url, options = {}) {
        const response = await fetch(url, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            }
        });

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            if (response.status === 401) throw new Error('Unauthorized: Please check your API token.');
            if (response.status === 403) throw new Error('Forbidden: You do not have permission to perform this action.');
            if (response.status === 409) throw new Error('Conflict: An item with this name already exists.');
            if (response.status === 400) throw new Error(error.error || 'Invalid data provided. Please check your inputs.');

            throw new Error(error.error || `Request failed with status ${response.status}`);
        }

        // For 204 No Content
        if (response.status === 204) return null;

        return await response.json();
    },
    get(url) { return this.request(url); },
    post(url, data) { return this.request(url, { method: 'POST', body: JSON.stringify(data) }); },
    put(url, data) { return this.request(url, { method: 'PUT', body: JSON.stringify(data) }); },
    delete(url) { return this.request(url, { method: 'DELETE' }); }
};

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

    // loadDatacenters delegates to store

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

    // loadDatacenters and loadNetworks delegate to store

    // enrichNetworks handled by getter

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

Alpine.data('deviceManager', () => ({
    devices: [],
    get datacenters() { return Alpine.store('appData').datacenters; },
    get networks() { return Alpine.store('appData').networks; },
    localLoading: false,
    get loading() { return this.localLoading; }, // Devices loading is local
    saving: false,
    showModal: false,
    showViewModal: false,
    searchQuery: '',
    modalTitle: 'Add Device',
    currentDevice: {},
    form: {
        id: '', name: '', description: '', make_model: '', os: '',
        datacenter_id: '', username: '', location: '', tagsInput: '', domainsInput: '',
        addresses: []
    },

    init() {
        this.loadDevices();
        Alpine.store('appData').loadDatacenters();
        Alpine.store('appData').loadNetworks();

        window.addEventListener('refresh-datacenters', () => {
            Alpine.store('appData').loadDatacenters(true).then(() => this.loadDevices());
        });
        window.addEventListener('refresh-networks', () => {
            Alpine.store('appData').loadNetworks(true).then(() => this.loadDevices());
        });
    },

    // Check if there's only one datacenter
    get hasSingleDatacenter() {
        return this.datacenters.length === 1;
    },

    // Get the single datacenter ID if there's only one
    get singleDatacenterId() {
        return this.hasSingleDatacenter ? this.datacenters[0].id : null;
    },

    async ensureDependencies() {
        await Promise.all([
            Alpine.store('appData').loadNetworks(),
            Alpine.store('appData').loadDatacenters()
        ]);
        // Always wait to ensure DOM options are rendered and ready for binding
        await new Promise(resolve => setTimeout(resolve, 50));
    },

    // loadDatacenters and loadNetworks removed

    async loadDevices() {
        this.localLoading = true;
        try {
            const url = this.searchQuery
                ? `/api/search?q=${encodeURIComponent(this.searchQuery)}`
                : '/api/devices';
            const data = await api.get(url);
            this.devices = Array.isArray(data) ? data : [];
            this.enrichDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to load devices', 'error');
            this.devices = [];
        } finally {
            this.localLoading = false;
        }
    },

    enrichDevices() {
        if (!this.devices) return;
        this.devices = this.devices.map(device => {
            const enriched = {
                ...device,
                datacenter_name: this.datacenters.find(dc => dc.id === device.datacenter_id)?.name || null
            };
            if (enriched.addresses) {
                enriched.addresses = enriched.addresses.map(addr => ({
                    ...addr,
                    network_name: this.networks.find(n => n.id === addr.network_id)?.name || null
                }));
            }
            return enriched;
        });
    },

    clearSearch() {
        this.searchQuery = '';
        this.loadDevices();
    },

    async openAddModal() {
        await this.ensureDependencies();
        this.modalTitle = 'Add Device';
        this.resetForm();
        this.$nextTick(() => {
            this.showModal = true;
        });
    },

    closeModal() {
        this.showModal = false;
        this.resetForm();
    },

    resetForm() {
        this.form = {
            id: '', name: '', description: '', make_model: '', os: '',
            datacenter_id: '', username: '', location: '', tagsInput: '', domainsInput: '',
            addresses: [{ ip: '', port: '', type: 'ipv4', label: '', network_id: '', pool_id: '', switch_port: '' }]
        };
        // Auto-select the single datacenter if there's only one
        if (this.hasSingleDatacenter) {
            this.form.datacenter_id = this.singleDatacenterId;
        }
    },

    addAddress() {
        this.form.addresses.push({ ip: '', port: '', type: 'ipv4', label: '', network_id: '', pool_id: '', switch_port: '' });
    },

    removeAddress(index) {
        this.form.addresses.splice(index, 1);
        if (this.form.addresses.length === 0) {
            this.addAddress();
        }
    },

    async saveDevice() {
        this.saving = true;
        try {
            const addresses = this.form.addresses
                .filter(a => a.ip)
                .map(a => ({
                    ip: a.ip,
                    port: a.port && a.port !== '' ? parseInt(a.port, 10) : 0,
                    type: a.type || 'ipv4',
                    label: a.label || '',
                    network_id: a.network_id || '',
                    pool_id: a.pool_id || '',
                    switch_port: a.switch_port || ''
                }));

            const payload = {
                name: this.form.name,
                description: this.form.description || '',
                make_model: this.form.make_model || '',
                os: this.form.os || '',
                datacenter_id: this.form.datacenter_id || '',
                username: this.form.username || '',
                location: this.form.location || '',
                tags: this.form.tagsInput.split(',').map(t => t.trim()).filter(t => t),
                domains: this.form.domainsInput.split(',').map(t => t.trim()).filter(t => t),
                addresses: addresses
            };

            if (this.form.id) {
                await api.put(`/api/devices/${this.form.id}`, payload);
                Alpine.store('toast').notify('Device updated successfully', 'success');
            } else {
                await api.post('/api/devices', payload);
                Alpine.store('toast').notify('Device created successfully', 'success');
            }

            this.closeModal();
            this.loadDevices();
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async viewDevice(id) {
        try {
            const device = await api.get(`/api/devices/${id}`);
            device.datacenter_name = this.datacenters.find(dc => dc.id === device.datacenter_id)?.name || null;
            if (device.addresses) {
                device.addresses = device.addresses.map(addr => ({
                    ...addr,
                    network_name: this.networks.find(n => n.id === addr.network_id)?.name || null
                }));
            }
            this.currentDevice = device;
            this.showViewModal = true;
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    closeViewModal() {
        this.showViewModal = false;
        this.currentDevice = {};
    },

    async editCurrentDevice() {
        await this.ensureDependencies();
        const device = this.currentDevice;
        await this.prepareEditForm(device); // Await form preparation including pool fetches
        this.closeViewModal();
        this.showModal = true;
    },

    async editDevice(id) {
        try {
            await this.ensureDependencies();
            const device = await api.get(`/api/devices/${id}`);
            await this.prepareEditForm(device); // Await form preparation including pool fetches
            this.showModal = true;
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    async prepareEditForm(device) {
        this.modalTitle = 'Edit Device';
        const addresses = device.addresses && device.addresses.length > 0
            ? device.addresses.map(a => ({
                ...a,
                network_id: a.network_id || '',
                pool_id: a.pool_id || '',
                port: a.port === 0 ? '' : a.port // Display 0 as empty string
            }))
            : [{ ip: '', port: '', type: 'ipv4', label: '', network_id: '', pool_id: '', switch_port: '' }];

        // Pre-load pools for existing networks BEFORE setting the form
        // This ensures options are available when Alpine renders the select
        const networkIds = [...new Set(addresses.map(a => a.network_id).filter(id => id))];
        await Promise.all(networkIds.map(id => this.fetchPoolsForNetwork(id)));

        this.form = {
            id: device.id || '',
            name: device.name || '',
            description: device.description || '',
            make_model: device.make_model || '',
            os: device.os || '',
            datacenter_id: device.datacenter_id || '',
            username: device.username || '',
            location: device.location || '',
            tagsInput: (device.tags || []).join(', '),
            domainsInput: (device.domains || []).join(', '),
            addresses: addresses
        };
    },

    async deleteDevice(id) {
        if (!confirm('Are you sure you want to delete this device?')) return;
        try {
            await api.delete(`/api/devices/${id}`);
            Alpine.store('toast').notify('Device deleted successfully', 'success');
            this.loadDevices();
            if (this.showViewModal && this.currentDevice.id === id) {
                this.closeViewModal();
            }
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete device', 'error');
        }
    },

    deleteCurrentDevice() {
        this.deleteDevice(this.currentDevice.id);
    },

    // Pool Support
    async getNextIP(poolId, index) {
        if (!poolId) return;
        try {
            const data = await api.get(`/api/pools/${poolId}/next-ip`);
            if (data.ip) {
                this.form.addresses[index].ip = data.ip;
                Alpine.store('toast').notify('IP address suggested', 'success');
            }
        } catch (error) {
            Alpine.store('toast').notify(error.message || 'Failed to get next IP', 'error');
        }
    },


    async loadPools(networkId) {
        if (!networkId) return [];
        try {
            const data = await api.get(`/api/networks/${networkId}/pools`);
            return Array.isArray(data) ? data : [];
        } catch (error) {
            console.error('Failed to load pools', error);
            return [];
        }
    },



    availablePools: {}, // Map of networkId -> pools
    async fetchPoolsForNetwork(networkId) {
        if (!networkId || this.availablePools[networkId]) return;
        const pools = await this.loadPools(networkId);
        // Use spread to ensure reactivity when adding new property
        this.availablePools = { ...this.availablePools, [networkId]: pools };
    }
}));

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