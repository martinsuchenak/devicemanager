import Alpine from 'alpinejs';
import { api } from './api.js';
import { modalConfig, viewModalConfig } from './modal.js';

Alpine.data('discoveryManager', () => ({
    // Reusable modal configurations
    viewModal: viewModalConfig('2xl'),
    promoteModal: modalConfig('2xl'),
    bulkPromoteModal: modalConfig('2xl'),
    scanModal: modalConfig('md'),

    discoveredDevices: [],
    scans: [],
    rules: [],
    get networks() { return Alpine.store('appData').networks; },
    localLoading: false,
    get loading() { return this.localLoading; },
    saving: false,
    scanning: false,
    // For backward compatibility with existing HTML
    get showPromoteModal() { return this.promoteModal.show; },
    set showPromoteModal(value) { this.promoteModal.show = value; },
    get showBulkPromoteModal() { return this.bulkPromoteModal.show; },
    set showBulkPromoteModal(value) { this.bulkPromoteModal.show = value; },
    get showViewModal() { return this.viewModal.show; },
    set showViewModal(value) { this.viewModal.show = value; },
    get showScanModal() { return this.scanModal.show; },
    set showScanModal(value) { this.scanModal.show = value; },
    selectedDevices: new Set(),
    filters: {
        network_id: '',
        status: '',
        promoted: ''
    },
    currentDevice: {},
    promoteForm: {
        name: '',
        description: '',
        make_model: '',
        os: '',
        datacenter_id: '',
        username: '',
        location: '',
        tagsInput: '',
        domainsInput: ''
    },
    bulkPromoteForm: {
        name: '',
        description: '',
        make_model: '',
        os: '',
        datacenter_id: '',
        username: '',
        location: '',
        tagsInput: '',
        domainsInput: ''
    },
    scanForm: {
        network_id: ''
    },
    ruleForm: {
        id: '',
        network_id: '',
        enabled: true,
        scan_interval_hours: 24,
        exclude_ips: ''
    },

    init() {
        this.loadDiscoveredDevices();
        this.loadScans();
        this.loadRules();
        Alpine.store('appData').loadNetworks();
    },

    async loadDiscoveredDevices() {
        this.localLoading = true;
        try {
            const params = new URLSearchParams();
            if (this.filters.network_id) params.append('network_id', this.filters.network_id);
            if (this.filters.status) params.append('status', this.filters.status);
            if (this.filters.promoted !== '') params.append('promoted', this.filters.promoted);

            const url = `/api/discovered${params.toString() ? '?' + params.toString() : ''}`;
            const data = await api.get(url);
            this.discoveredDevices = Array.isArray(data) ? data : [];
            this.enrichDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to load discovered devices', 'error');
            this.discoveredDevices = [];
        } finally {
            this.localLoading = false;
        }
    },

    enrichDevices() {
        if (!this.discoveredDevices) return;
        this.discoveredDevices = this.discoveredDevices.map(device => ({
            ...device,
            network_name: this.networks.find(n => n.id === device.network_id)?.name || null
        }));
    },

    async loadScans() {
        try {
            const data = await api.get('/api/discovery/scans');
            this.scans = Array.isArray(data) ? data : [];
        } catch (error) {
            this.scans = [];
        }
    },

    async loadRules() {
        try {
            const data = await api.get('/api/discovery/rules');
            this.rules = Array.isArray(data) ? data : [];
        } catch (error) {
            this.rules = [];
        }
    },

    applyFilters() {
        this.loadDiscoveredDevices();
    },

    clearFilters() {
        this.filters = { network_id: '', status: '', promoted: '' };
        this.loadDiscoveredDevices();
    },

    async viewDevice(id) {
        try {
            const device = await api.get(`/api/discovered/${id}`);
            device.network_name = this.networks.find(n => n.id === device.network_id)?.name || null;
            this.viewModal.openWithItem(device);
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    closeViewModal() {
        this.viewModal.close();
    },

    async openPromoteModal(id) {
        try {
            const device = await api.get(`/api/discovered/${id}`);
            this.promoteForm = {
                name: device.hostname || device.ip,
                description: `Auto-discovered from ${device.ip}`,
                make_model: '',
                os: device.os_guess || '',
                datacenter_id: '',
                username: '',
                location: '',
                tagsInput: 'discovered',
                domainsInput: ''
            };
            // Store current device for promoteDevice method
            this.viewModal.currentItem = device;
            this.promoteModal.open();
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    closePromoteModal() {
        this.promoteModal.close();
        this.resetPromoteForm();
    },

    resetPromoteForm() {
        this.promoteForm = {
            name: '',
            description: '',
            make_model: '',
            os: '',
            datacenter_id: '',
            username: '',
            location: '',
            tagsInput: '',
            domainsInput: ''
        };
    },

    async promoteDevice() {
        this.saving = true;
        try {
            await api.post(`/api/discovered/${this.viewModal.currentItem.id}/promote`, this.promoteForm);
            Alpine.store('toast').notify('Device promoted successfully', 'success');
            this.closePromoteModal();
            this.loadDiscoveredDevices();
            window.dispatchEvent(new CustomEvent('refresh-devices'));
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    toggleSelectDevice(id) {
        if (this.selectedDevices.has(id)) {
            this.selectedDevices.delete(id);
        } else {
            this.selectedDevices.add(id);
        }
    },

    toggleSelectAll() {
        if (this.selectedDevices.size === this.discoveredDevices.length) {
            this.selectedDevices.clear();
        } else {
            this.discoveredDevices.forEach(d => this.selectedDevices.add(d.id));
        }
    },

    get selectedDevicesList() {
        return Array.from(this.selectedDevices);
    },

    get hasSelectedDevices() {
        return this.selectedDevices.size > 0;
    },

    async openBulkPromoteModal() {
        if (this.selectedDevices.size === 0) return;
        this.bulkPromoteForm = {
            name: '',
            description: 'Bulk promoted discovered devices',
            make_model: '',
            os: '',
            datacenter_id: '',
            username: '',
            location: '',
            tagsInput: 'discovered',
            domainsInput: ''
        };
        this.bulkPromoteModal.open();
    },

    closeBulkPromoteModal() {
        this.bulkPromoteModal.close();
        this.selectedDevices.clear();
    },

    async bulkPromote() {
        this.saving = true;
        try {
            const ids = this.selectedDevicesList;
            const devices = ids.map(() => ({
                name: this.bulkPromoteForm.name || undefined,
                description: this.bulkPromoteForm.description,
                make_model: this.bulkPromoteForm.make_model,
                os: this.bulkPromoteForm.os,
                datacenter_id: this.bulkPromoteForm.datacenter_id,
                username: this.bulkPromoteForm.username,
                location: this.bulkPromoteForm.location,
                tags: this.bulkPromoteForm.tagsInput.split(',').map(t => t.trim()).filter(t => t),
                domains: this.bulkPromoteForm.domainsInput.split(',').map(t => t.trim()).filter(t => t)
            }));

            await api.post('/api/discovered/bulk-promote', { ids, devices });
            Alpine.store('toast').notify(`${ids.length} device(s) promoted successfully`, 'success');
            this.closeBulkPromoteModal();
            this.loadDiscoveredDevices();
            window.dispatchEvent(new CustomEvent('refresh-devices'));
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async deleteDevice(id) {
        if (!confirm('Are you sure you want to delete this discovered device?')) return;
        try {
            await api.delete(`/api/discovered/${id}`);
            Alpine.store('toast').notify('Device deleted successfully', 'success');
            // Close modal if we deleted the currently viewed device
            if (this.viewModal.show && this.viewModal.currentItem?.id === id) {
                this.viewModal.close();
            }
            this.loadDiscoveredDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete device', 'error');
        }
    },

    deleteCurrentDevice() {
        this.deleteDevice(this.viewModal.currentItem?.id);
    },

    async bulkDelete() {
        if (!confirm(`Are you sure you want to delete ${this.selectedDevices.size} discovered device(s)?`)) return;
        try {
            await Promise.all(this.selectedDevicesList.map(id => api.delete(`/api/discovered/${id}`)));
            Alpine.store('toast').notify(`${this.selectedDevices.size} device(s) deleted successfully`, 'success');
            this.selectedDevices.clear();
            this.loadDiscoveredDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete devices', 'error');
        }
    },

    async deleteAll() {
        const count = this.discoveredDevices.length;
        if (count === 0) {
            Alpine.store('toast').notify('No devices to delete', 'error');
            return;
        }
        if (!confirm(`Are you sure you want to delete ALL ${count} discovered device(s)? This cannot be undone.`)) return;
        try {
            await Promise.all(this.discoveredDevices.map(device => api.delete(`/api/discovered/${device.id}`)));
            Alpine.store('toast').notify(`All ${count} device(s) deleted successfully`, 'success');
            this.loadDiscoveredDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete all devices', 'error');
        }
    },

    openScanModal() {
        this.scanForm = {
            network_id: this.filters.network_id || ''
        };
        this.scanModal.open();
    },

    closeScanModal() {
        this.scanModal.close();
    },

    async startScan() {
        this.scanning = true;
        try {
            await api.post('/api/discovery/scans', this.scanForm);
            Alpine.store('toast').notify('Discovery scan started', 'success');
            this.closeScanModal();
            this.loadScans();
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.scanning = false;
        }
    },

    async deleteScan(id) {
        if (!confirm('Are you sure you want to delete this scan?')) return;
        try {
            await api.delete(`/api/discovery/scans/${id}`);
            Alpine.store('toast').notify('Scan deleted successfully', 'success');
            this.loadScans();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete scan', 'error');
        }
    },

    openRuleModal(rule = null) {
        if (rule) {
            this.ruleForm = {
                ...rule,
                exclude_ips: rule.exclude_ips ? rule.exclude_ips.join(', ') : ''
            };
        } else {
            this.ruleForm = {
                id: '',
                network_id: '',
                enabled: true,
                scan_interval_hours: 24,
                exclude_ips: ''
            };
        }
        this.showRuleModal = true;
    },

    closeRuleModal() {
        this.showRuleModal = false;
        this.resetRuleForm();
    },

    resetRuleForm() {
        this.ruleForm = {
            id: '',
            network_id: '',
            enabled: true,
            scan_interval_hours: 24,
            exclude_ips: ''
        };
    },

    async saveRule() {
        this.saving = true;
        try {
            const payload = {
                network_id: this.ruleForm.network_id,
                enabled: this.ruleForm.enabled,
                scan_interval_hours: this.ruleForm.scan_interval_hours,
                scan_type: 'basic',
                exclude_ips: this.ruleForm.exclude_ips.split(',').map(t => t.trim()).filter(t => t)
            };

            if (this.ruleForm.id) {
                await api.put(`/api/discovery/rules/${this.ruleForm.id}`, payload);
                Alpine.store('toast').notify('Rule updated successfully', 'success');
            } else {
                await api.post('/api/discovery/rules', payload);
                Alpine.store('toast').notify('Rule created successfully', 'success');
            }

            this.closeRuleModal();
            this.loadRules();
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async deleteRule(id) {
        if (!confirm('Are you sure you want to delete this rule?')) return;
        try {
            await api.delete(`/api/discovery/rules/${id}`);
            Alpine.store('toast').notify('Rule deleted successfully', 'success');
            this.loadRules();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete rule', 'error');
        }
    },

    // Status badge helper
    getStatusBadgeClass(status) {
        const classes = {
            online: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
            offline: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
            unknown: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
        };
        return classes[status] || classes.unknown;
    },

    getConfidenceBadgeClass(confidence) {
        if (confidence >= 80) return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
        if (confidence >= 50) return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200';
        return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
    }
}));
