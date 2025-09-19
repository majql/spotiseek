class SpotiseekAPI {
    constructor(baseURL = '') {
        this.baseURL = baseURL;
    }

    async request(endpoint, options = {}) {
        try {
            const response = await fetch(`${this.baseURL}/api${endpoint}`, {
                headers: {
                    'Content-Type': 'application/json',
                    ...options.headers
                },
                ...options
            });

            const data = await response.json();

            if (!data.success) {
                throw new Error(data.error || 'Unknown error');
            }

            return data.data;
        } catch (error) {
            if (error instanceof TypeError && error.message.includes('fetch')) {
                throw new Error('Unable to connect to Spotiseek server');
            }
            throw error;
        }
    }

    async getStatus() {
        return await this.request('/status');
    }

    async watchPlaylist(playlist, backfill = false) {
        return await this.request('/watch', {
            method: 'POST',
            body: JSON.stringify({ playlist, backfill })
        });
    }

    async forgetPlaylist(playlistId) {
        return await this.request(`/forget/${encodeURIComponent(playlistId)}`, {
            method: 'DELETE'
        });
    }
}

class SpotiseekUI {
    constructor() {
        this.api = new SpotiseekAPI();
        this.elements = {
            addForm: document.getElementById('add-playlist-form'),
            playlistInput: document.getElementById('playlist-input'),
            watchBtn: document.getElementById('watch-btn'),
            watchBackfillBtn: document.getElementById('watch-backfill-btn'),
            refreshBtn: document.getElementById('refresh-btn'),
            loading: document.getElementById('loading'),
            errorMessage: document.getElementById('error-message'),
            successMessage: document.getElementById('success-message'),
            playlistCount: document.getElementById('playlist-count'),
            playlistsContainer: document.getElementById('playlists-container')
        };

        this.bindEvents();
        this.loadStatus();
    }

    bindEvents() {
        this.elements.watchBtn.addEventListener('click', (e) => this.handleAddPlaylist(e, false));
        this.elements.watchBackfillBtn.addEventListener('click', (e) => this.handleAddPlaylist(e, true));
        this.elements.refreshBtn.addEventListener('click', () => this.loadStatus());
    }

    showLoading() {
        this.elements.loading.classList.remove('hidden');
    }

    hideLoading() {
        this.elements.loading.classList.add('hidden');
    }

    showError(message) {
        this.elements.errorMessage.textContent = message;
        this.elements.errorMessage.classList.remove('hidden');
        setTimeout(() => this.elements.errorMessage.classList.add('hidden'), 5000);
    }

    showSuccess(message) {
        this.elements.successMessage.textContent = message;
        this.elements.successMessage.classList.remove('hidden');
        setTimeout(() => this.elements.successMessage.classList.add('hidden'), 3000);
    }

    async handleAddPlaylist(e, backfill) {
        e.preventDefault();

        const playlist = this.elements.playlistInput.value.trim();
        if (!playlist) {
            this.showError('Please enter a playlist ID or URL');
            return;
        }

        // Disable both buttons and update text
        this.elements.watchBtn.disabled = true;
        this.elements.watchBackfillBtn.disabled = true;

        const originalWatchText = this.elements.watchBtn.textContent;
        const originalBackfillText = this.elements.watchBackfillBtn.textContent;

        if (backfill) {
            this.elements.watchBackfillBtn.textContent = 'Adding...';
        } else {
            this.elements.watchBtn.textContent = 'Adding...';
        }

        try {
            const result = await this.api.watchPlaylist(playlist, backfill);
            this.showSuccess(result.message);
            this.elements.playlistInput.value = '';
            await this.loadStatus();
        } catch (error) {
            this.showError(error.message);
        } finally {
            // Re-enable both buttons and restore text
            this.elements.watchBtn.disabled = false;
            this.elements.watchBackfillBtn.disabled = false;
            this.elements.watchBtn.textContent = originalWatchText;
            this.elements.watchBackfillBtn.textContent = originalBackfillText;
        }
    }

    async handleForgetPlaylist(playlistId) {
        if (!confirm('Are you sure you want to stop watching this playlist?')) {
            return;
        }

        try {
            const result = await this.api.forgetPlaylist(playlistId);
            this.showSuccess(result.message);
            await this.loadStatus();
        } catch (error) {
            this.showError(error.message);
        }
    }

    async loadStatus() {
        this.showLoading();

        try {
            const status = await this.api.getStatus();
            this.updateUI(status);
        } catch (error) {
            this.showError(error.message);
        } finally {
            this.hideLoading();
        }
    }

    updateUI(status) {
        // Update count
        const count = status.count || 0;
        this.elements.playlistCount.textContent =
            count === 0 ? 'No playlists being watched' :
            count === 1 ? '1 playlist being watched' :
            `${count} playlists being watched`;

        // Clear container
        this.elements.playlistsContainer.innerHTML = '';

        if (count === 0) {
            this.elements.playlistsContainer.innerHTML = `
                <div class="empty-state">
                    <p>No playlists are currently being watched.</p>
                    <p>Add a Spotify playlist above to get started!</p>
                </div>
            `;
            return;
        }

        // Add playlists
        status.playlists.forEach(playlist => {
            const playlistElement = this.createPlaylistElement(playlist);
            this.elements.playlistsContainer.appendChild(playlistElement);
        });
    }

    createPlaylistElement(playlist) {
        const div = document.createElement('div');
        div.className = 'playlist-card';

        const statusClass = this.getStatusClass(playlist.status);
        const statusIcon = this.getStatusIcon(playlist.status);

        div.innerHTML = `
            <div class="playlist-header">
                <div class="playlist-info">
                    <h3 class="playlist-id">${playlist.playlist_name ? this.escapeHtml(playlist.playlist_name) + ' (' + this.escapeHtml(playlist.playlist_id) + ')' : this.escapeHtml(playlist.playlist_id)}</h3>
                    <div class="playlist-status ${statusClass}">
                        ${statusIcon} ${this.escapeHtml(playlist.status)}
                    </div>
                </div>
                <button class="forget-btn" onclick="app.handleForgetPlaylist('${this.escapeHtml(playlist.playlist_id)}')">
                    🗑️ Stop Watching
                </button>
            </div>
            <div class="playlist-details">
                <div class="detail-item">
                    <strong>Created:</strong> ${this.formatDate(playlist.created_at)}
                </div>
                <div class="detail-item">
                    <strong>Worker:</strong> ${this.escapeHtml(playlist.worker_name)}
                </div>
                <div class="detail-item">
                    <strong>Slskd:</strong> ${this.escapeHtml(playlist.slskd_name)}
                </div>
                <div class="detail-item">
                    <strong>Network:</strong> ${this.escapeHtml(playlist.network_name)}
                </div>
            </div>
        `;

        return div;
    }

    getStatusClass(status) {
        switch (status.toLowerCase()) {
            case 'running':
                return 'status-running';
            case 'stopped':
                return 'status-stopped';
            case 'error':
                return 'status-error';
            default:
                return 'status-unknown';
        }
    }

    getStatusIcon(status) {
        switch (status.toLowerCase()) {
            case 'running':
                return '🟢';
            case 'stopped':
                return '🔴';
            case 'error':
                return '⚠️';
            default:
                return '⚪';
        }
    }

    formatDate(dateStr) {
        try {
            return new Date(dateStr).toLocaleString();
        } catch {
            return dateStr;
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize the application
let app;
document.addEventListener('DOMContentLoaded', () => {
    app = new SpotiseekUI();
});