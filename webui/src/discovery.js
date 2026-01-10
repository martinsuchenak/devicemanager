import Alpine from 'alpinejs';
import { api } from './api.js';

Alpine.data('discoveryManager', () => ({
    discoveredDevices: [],
    scans: [],
    rules: [],
    get networks() { return Alpine.store('appData').networks; },
    localLoading: false,
    get loading() { return this.localLoading; },
    saving: false,
    scanning: false,
    showPromoteModal: false,
    showBulkPromoteModal: false,
    showViewModal: false,
    showScanModal: false,
    showRuleModal: false,
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
        network_id: '',
        scan_type: 'full'
    },
    ruleForm: {
        id: '',
        network_id: '',
        enabled: true,
        scan_interval_hours: 24,
        scan_type: 'full',
        max_concurrent_scans: 10,
        timeout_seconds: 5,
        scan_ports: true,
        port_scan_type: 'common',
        service_detection: true,
        os_detection: true,
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

    async openPromoteModal(id) {
        try {
            const device = await api.get(`/api/discovered/${id}`);
            this.currentDevice = device;
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
            this.showPromoteModal = true;
        } catch (error) {
            Alpine.store('toast').notify('Failed to load device', 'error');
        }
    },

    closePromoteModal() {
        this.showPromoteModal = false;
        this.currentDevice = {};
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
            await api.post(`/api/discovered/${this.currentDevice.id}/promote`, this.promoteForm);
            Alpine.store('toast').notify('Device promoted successfully', 'success');
            this.closePromoteModal();
            this.loadDiscoveredDevices();
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
        this.showBulkPromoteModal = true;
    },

    closeBulkPromoteModal() {
        this.showBulkPromoteModal = false;
        this.selectedDevices.clear();
    },

    async bulkPromote() {
        this.saving = true;
        try {
            const ids = this.selectedDevicesList;
            const devices = ids.map(id => ({
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
        } catch (error) {
            Alpine.store('toast').notify(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async deleteDevice(id) {
        if (!confirm('Are you sure you want to delete this discovered device?')) return;
        try {
            await api.deleteReq(`/api/discovered/${id}`);
            Alpine.store('toast').notify('Device deleted successfully', 'success');
            this.loadDiscoveredDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete device', 'error');
        }
    },

    async bulkDelete() {
        if (!confirm(`Are you sure you want to delete ${this.selectedDevices.size} discovered device(s)?`)) return;
        try {
            await Promise.all(this.selectedDevicesList.map(id => api.deleteReq(`/api/discovered/${id}`)));
            Alpine.store('toast').notify(`${this.selectedDevices.size} device(s) deleted successfully`, 'success');
            this.selectedDevices.clear();
            this.loadDiscoveredDevices();
        } catch (error) {
            Alpine.store('toast').notify('Failed to delete devices', 'error');
        }
    },

    openScanModal() {
        this.scanForm = {
            network_id: this.filters.network_id || '',
            scan_type: 'full'
        };
        this.showScanModal = true;
    },

    closeScanModal() {
        this.showScanModal = false;
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
            await api.deleteReq(`/api/discovery/scans/${id}`);
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
                scan_type: 'full',
                max_concurrent_scans: 10,
                timeout_seconds: 5,
                scan_ports: true,
                port_scan_type: 'common',
                service_detection: true,
                os_detection: true,
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
            scan_type: 'full',
            max_concurrent_scans: 10,
            timeout_seconds: 5,
            scan_ports: true,
            port_scan_type: 'common',
            service_detection: true,
            os_detection: true,
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
                scan_type: this.ruleForm.scan_type,
                max_concurrent_scans: this.ruleForm.max_concurrent_scans,
                timeout_seconds: this.ruleForm.timeout_seconds,
                scan_ports: this.ruleForm.scan_ports,
                port_scan_type: this.ruleForm.port_scan_type,
                service_detection: this.ruleForm.service_detection,
                os_detection: this.ruleForm.os_detection,
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
            await api.deleteReq(`/api/discovery/rules/${id}`);
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
