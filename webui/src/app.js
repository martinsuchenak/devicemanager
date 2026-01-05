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

Alpine.data('datacenterManager', () => ({
    datacenters: [],
    loading: false,
    saving: false,
    showModal: false,
    showViewModal: false,
    modalTitle: 'Add Datacenter',
    currentDatacenter: {},
    form: { id: '', name: '', location: '', description: '' },

    init() {
        this.loadDatacenters();
        // Listen for refresh events
        window.addEventListener('refresh-datacenters', () => this.loadDatacenters());
    },

    async loadDatacenters() {
        this.loading = true;
        try {
            const data = await api.get('/api/datacenters');
            this.datacenters = Array.isArray(data) ? data : [];
        } catch (error) {
            Alpine.store('toast').notify('Failed to load datacenters', 'error');
            this.datacenters = [];
        } finally {
            this.loading = false;
        }
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
            this.loadDatacenters();
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
            this.loadDatacenters();
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
    networks: [],
    datacenters: [],
    loading: false,
    saving: false,
    showModal: false,
    showViewModal: false,
    modalTitle: 'Add Network',
    currentNetwork: {},
    form: { id: '', name: '', subnet: '', datacenter_id: '', description: '' },

    init() {
        this.loadNetworks();
        this.loadDatacenters();
        window.addEventListener('refresh-networks', () => this.loadNetworks());
        window.addEventListener('refresh-datacenters', () => this.loadDatacenters());
    },

    // Check if there's only one datacenter
    get hasSingleDatacenter() {
        return this.datacenters.length === 1;
    },

    // Get the single datacenter ID if there's only one
    get singleDatacenterId() {
        return this.hasSingleDatacenter ? this.datacenters[0].id : null;
    },

    async loadDatacenters() {
        this.loading = true;
        try {
            const data = await api.get('/api/datacenters');
            this.datacenters = Array.isArray(data) ? data : [];
        } catch (error) {
            Alpine.store('toast').notify('Failed to load datacenters', 'error');
            this.datacenters = [];
        } finally {
            this.loading = false;
        }
    },


    async loadNetworks() {
        this.loading = true;
        try {
            const data = await api.get('/api/networks');
            this.networks = Array.isArray(data) ? data : [];
            this.enrichNetworks();
        } catch (error) {
            Alpine.store('toast').notify('Failed to load networks', 'error');
            this.networks = [];
        } finally {
            this.loading = false;
        }
    },

    enrichNetworks() {
        if (!this.networks.length || !this.datacenters.length) return;
        this.networks = this.networks.map(network => ({
            ...network,
            datacenter_name: this.datacenters.find(dc => dc.id === network.datacenter_id)?.name || null
        }));
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
            this.loadNetworks();
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
            this.loadNetworks();
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
    datacenters: [],
    networks: [],
    loading: false,
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
        this.loadDatacenters();
        this.loadNetworks();

        window.addEventListener('refresh-datacenters', () => {
            this.loadDatacenters();
            this.loadDevices(); // Reload devices to update datacenter names
        });
        window.addEventListener('refresh-networks', () => {
            this.loadNetworks();
            this.loadDevices(); // Reload devices to update network names
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
        const promises = [];
        if (!this.networks || this.networks.length === 0) {
            promises.push(this.loadNetworks());
        }
        if (!this.datacenters || this.datacenters.length === 0) {
            promises.push(this.loadDatacenters());
        }
        if (promises.length > 0) {
            await Promise.all(promises);
        }
        // Always wait to ensure DOM options are rendered and ready for binding
        await new Promise(resolve => setTimeout(resolve, 50));
    },

    async loadDatacenters() {
        try {
            const data = await api.get('/api/datacenters');
            this.datacenters = Array.isArray(data) ? data : [];
            this.enrichDevices();
        } catch (error) {
            console.error('Failed to load datacenters', error);
            this.datacenters = [];
        }
    },

    async loadNetworks() {
        try {
            const data = await api.get('/api/networks');
            this.networks = Array.isArray(data) ? data : [];
            this.enrichDevices();
        } catch (error) {
            console.error('Failed to load networks', error);
            this.networks = [];
        }
    },

    async loadDevices() {
        this.loading = true;
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
            this.loading = false;
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
            addresses: [{ ip: '', port: '', type: 'ipv4', label: '', network_id: '', switch_port: '' }]
        };
        // Auto-select the single datacenter if there's only one
        if (this.hasSingleDatacenter) {
            this.form.datacenter_id = this.singleDatacenterId;
        }
    },

    addAddress() {
        this.form.addresses.push({ ip: '', port: '', type: 'ipv4', label: '', network_id: '', switch_port: '' });
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
        this.showModal = true;
        this.closeViewModal();
        this.$nextTick(() => {
            this.prepareEditForm(device);
        });
    },

    async editDevice(id) {
        try {
            await this.ensureDependencies();
            const device = await api.get(`/api/devices/${id}`);
            this.showModal = true;
            this.$nextTick(() => {
                this.prepareEditForm(device);
            });
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    prepareEditForm(device) {
        this.modalTitle = 'Edit Device';
        const addresses = device.addresses && device.addresses.length > 0
            ? device.addresses.map(a => ({ ...a, network_id: a.network_id || '' }))
            : [{ ip: '', port: '', type: 'ipv4', label: '', network_id: '', switch_port: '' }];

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
    }
}));

Alpine.start();