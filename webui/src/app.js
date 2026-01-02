import Alpine from 'alpinejs';

Alpine.data('deviceManager', () => ({
    devices: [],
    loading: false,
    saving: false,
    showModal: false,
    showViewModal: false,
    searchQuery: '',
    modalTitle: 'Add Device',
    currentDevice: {},
    form: {
        id: '',
        name: '',
        description: '',
        make_model: '',
        os: '',
        location: '',
        tagsInput: '',
        domainsInput: '',
        addresses: [{ ip: '', port: '', type: 'ipv4', label: '' }]
    },
    toast: { show: false, message: '', type: 'info' },

    async init() {
        await this.loadDevices();
    },

    async loadDevices() {
        this.loading = true;
        try {
            const url = this.searchQuery
                ? `/api/search?q=${encodeURIComponent(this.searchQuery)}`
                : '/api/devices';
            const response = await fetch(url);
            if (!response.ok) throw new Error('Failed to load devices');
            this.devices = await response.json();
        } catch (error) {
            this.showToast('Failed to load devices', 'error');
        } finally {
            this.loading = false;
        }
    },

    clearSearch() {
        this.searchQuery = '';
        this.loadDevices();
    },

    openAddModal() {
        this.modalTitle = 'Add Device';
        this.resetForm();
        this.showModal = true;
    },

    closeModal() {
        this.showModal = false;
        this.resetForm();
    },

    resetForm() {
        this.form = {
            id: '',
            name: '',
            description: '',
            make_model: '',
            os: '',
            location: '',
            tagsInput: '',
            domainsInput: '',
            addresses: [{ ip: '', port: '', type: 'ipv4', label: '' }]
        };
    },

    addAddress() {
        this.form.addresses.push({ ip: '', port: '', type: 'ipv4', label: '' });
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
            const device = {
                name: this.form.name,
                description: this.form.description,
                make_model: this.form.make_model,
                os: this.form.os,
                location: this.form.location,
                tags: this.form.tagsInput.split(',').map(t => t.trim()).filter(t => t),
                domains: this.form.domainsInput.split(',').map(t => t.trim()).filter(t => t),
                addresses: this.form.addresses.filter(a => a.ip)
            };

            const url = this.form.id ? `/api/devices/${this.form.id}` : '/api/devices';
            const method = this.form.id ? 'PUT' : 'POST';

            const response = await fetch(url, {
                method,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(device)
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to save device');
            }

            this.showToast(this.form.id ? 'Device updated successfully' : 'Device created successfully', 'success');
            this.closeModal();
            this.loadDevices();
        } catch (error) {
            this.showToast(error.message, 'error');
        } finally {
            this.saving = false;
        }
    },

    async viewDevice(id) {
        try {
            const response = await fetch(`/api/devices/${id}`);
            if (!response.ok) throw new Error('Failed to load device');
            this.currentDevice = await response.json();
            this.showViewModal = true;
        } catch (error) {
            this.showToast('Failed to load device', 'error');
        }
    },

    closeViewModal() {
        this.showViewModal = false;
        this.currentDevice = {};
    },

    editCurrentDevice() {
        const device = this.currentDevice;
        this.modalTitle = 'Edit Device';
        this.form = {
            id: device.id || '',
            name: device.name || '',
            description: device.description || '',
            make_model: device.make_model || '',
            os: device.os || '',
            location: device.location || '',
            tagsInput: (device.tags || []).join(', '),
            domainsInput: (device.domains || []).join(', '),
            addresses: device.addresses && device.addresses.length > 0
                ? [...device.addresses]
                : [{ ip: '', port: '', type: 'ipv4', label: '' }]
        };
        this.closeViewModal();
        this.showModal = true;
    },

    async editDevice(id) {
        try {
            const response = await fetch(`/api/devices/${id}`);
            if (!response.ok) throw new Error('Failed to load device');
            const device = await response.json();
            this.modalTitle = 'Edit Device';
            this.form = {
                id: device.id || '',
                name: device.name || '',
                description: device.description || '',
                make_model: device.make_model || '',
                os: device.os || '',
                location: device.location || '',
                tagsInput: (device.tags || []).join(', '),
                domainsInput: (device.domains || []).join(', '),
                addresses: device.addresses && device.addresses.length > 0
                    ? [...device.addresses]
                    : [{ ip: '', port: '', type: 'ipv4', label: '' }]
            };
            this.showModal = true;
        } catch (error) {
            this.showToast('Failed to load device', 'error');
        }
    },

    async deleteDeviceFromList(id) {
        if (!confirm('Are you sure you want to delete this device?')) return;

        try {
            const response = await fetch(`/api/devices/${id}`, {
                method: 'DELETE'
            });

            if (!response.ok) throw new Error('Failed to delete device');

            this.showToast('Device deleted successfully', 'success');
            this.loadDevices();
        } catch (error) {
            this.showToast('Failed to delete device', 'error');
        }
    },

    async deleteCurrentDevice() {
        if (!confirm('Are you sure you want to delete this device?')) return;

        try {
            const response = await fetch(`/api/devices/${this.currentDevice.id}`, {
                method: 'DELETE'
            });

            if (!response.ok) throw new Error('Failed to delete device');

            this.showToast('Device deleted successfully', 'success');
            this.closeViewModal();
            this.loadDevices();
        } catch (error) {
            this.showToast('Failed to delete device', 'error');
        }
    },

    showToast(message, type = 'info') {
        this.toast = { show: true, message, type };
        setTimeout(() => { this.toast.show = false; }, 3000);
    }
}));

Alpine.start();
