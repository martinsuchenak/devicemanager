import Alpine from 'alpinejs';
import { api } from './api.js';
import { modalConfig, viewModalConfig } from './modal.js';

Alpine.data('deviceManager', () => ({
    // Reusable modal configurations
    editModal: modalConfig('3xl'),
    viewModal: viewModalConfig('lg'),

    devices: [],
    get datacenters() { return Alpine.store('appData').datacenters; },
    get networks() { return Alpine.store('appData').networks; },
    localLoading: false,
    get loading() { return this.localLoading; }, // Devices loading is local
    saving: false,
    // For backward compatibility with existing HTML
    get showModal() { return this.editModal.show; },
    set showModal(value) { this.editModal.show = value; },
    get showViewModal() { return this.viewModal.show; },
    set showViewModal(value) { this.viewModal.show = value; },
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
        window.addEventListener('refresh-devices', () => this.loadDevices());
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
            this.editModal.open();
        });
    },

    closeModal() {
        this.editModal.close();
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
            this.viewModal.openWithItem(device);
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    closeViewModal() {
        this.viewModal.close();
    },

    async editCurrentDevice() {
        await this.ensureDependencies();
        const device = this.viewModal.currentItem;
        await this.prepareEditForm(device);
        this.viewModal.close();
        this.editModal.open();
    },

    async editDevice(id) {
        try {
            await this.ensureDependencies();
            const device = await api.get(`/api/devices/${id}`);
            await this.prepareEditForm(device);
            this.editModal.open();
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
            if (this.viewModal.show && this.viewModal.currentItem?.id === id) {
                this.viewModal.close();
            }
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete device', 'error');
        }
    },

    deleteCurrentDevice() {
        this.deleteDevice(this.viewModal.currentItem?.id);
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