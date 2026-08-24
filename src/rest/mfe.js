class NotifServerMFE extends HTMLElement {
    constructor() {
        super();
        this.baseUrl = '';
        this.statusData = { healthy: false, status: 'Offline', version: 'N/A', timestamp: 0, details: { active_notifiers: '0' } };
        this.notifiersList = [];
        this.alertingConfig = {};
        this.supportedTypes = [];
    }

    async connectedCallback() {
        this.baseUrl = this.getAttribute('base-url') || 'http://localhost:3311';
        this.renderSkeleton();
        await this.loadAll();
    }

    renderSkeleton() {
        this.innerHTML = `
            <style>
                .mfe-container {
                    font-family: var(--font-sans, 'Inter', sans-serif);
                    color: var(--color-text-primary, #e2e8f0);
                    width: 100%;
                    max-width: var(--content-max-width, 1400px);
                    margin: 0 auto;
                    padding: clamp(1.25rem, 2.5vh, 2.25rem) clamp(1.25rem, 3vw, 2.5rem) 3.5rem;
                    box-sizing: border-box;
                }
                .mfe-header {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    margin-bottom: 24px;
                }
                .mfe-title h1 {
                    font-family: var(--font-display, inherit);
                    font-size: var(--font-size-2xl, 2rem);
                    font-weight: 700;
                    margin: 0 0 5px 0;
                    color: var(--color-text-primary, #1a202c);
                }
                .mfe-title p {
                    color: var(--color-text-secondary, #718096);
                    margin: 0;
                    font-size: var(--font-size-sm, 0.95rem);
                }
                .mfe-btn-group {
                    display: flex;
                    gap: 10px;
                }
                .mfe-btn {
                    padding: 10px 18px;
                    border-radius: 8px;
                    font-weight: 600;
                    font-size: var(--font-size-sm, 0.9rem);
                    cursor: pointer;
                    transition: all 0.2s;
                    border: none;
                    display: inline-flex;
                    align-items: center;
                    gap: 6px;
                }
                .mfe-btn-primary {
                    background: linear-gradient(135deg, #6B0DF2 0%, #B80DF2 100%);
                    color: white;
                }
                .mfe-btn-action {
                    background: linear-gradient(135deg, #36d1dc 0%, #5b86e5 100%);
                    color: white;
                }
                .mfe-btn-secondary {
                    background-color: var(--color-bg-surface, #edf2f7);
                    color: var(--color-text-primary, #4a5568);
                    border: 1px solid var(--color-bg-secondary, transparent);
                }
                .mfe-btn-secondary:hover {
                    background-color: var(--color-bg-secondary, #e2e8f0);
                }
                .mfe-btn:hover {
                    opacity: 0.95;
                    transform: translateY(-1px);
                }
                
                /* Cards layout */
                .mfe-row {
                    display: flex;
                    gap: 20px;
                    margin-bottom: 24px;
                    flex-wrap: wrap;
                }
                .mfe-col-4 { flex: 1; min-width: 250px; }
                .mfe-col-8 { flex: 2; min-width: 320px; }
                
                .mfe-card {
                    background: var(--color-bg-surface, white);
                    border-radius: 12px;
                    box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -1px rgba(0,0,0,0.06);
                    padding: 20px;
                    border: 1px solid var(--color-bg-secondary, #edf2f7);
                }
                .mfe-card-title {
                    font-size: var(--font-size-xs, 0.75rem);
                    text-transform: uppercase;
                    color: var(--color-text-muted, #718096);
                    font-weight: 700;
                    margin-bottom: 12px;
                    letter-spacing: 0.05em;
                }
                .mfe-status-indicator {
                    display: flex;
                    align-items: center;
                    gap: 8px;
                    font-size: var(--font-size-lg, 1.25rem);
                    font-weight: 700;
                    margin-bottom: 10px;
                    color: var(--color-text-primary, inherit);
                }
                .mfe-dot {
                    height: 10px;
                    width: 10px;
                    border-radius: 50%;
                    display: inline-block;
                }
                .mfe-dot-active {
                    background-color: var(--color-accent-success, #48bb78);
                    box-shadow: 0 0 8px var(--color-accent-success, #48bb78);
                }
                .mfe-dot-inactive {
                    background-color: var(--color-accent-danger, #f56565);
                    box-shadow: 0 0 8px var(--color-accent-danger, #f56565);
                }
                
                .mfe-badge {
                    background-color: var(--color-bg-primary, #ebf8ff);
                    color: var(--color-accent-primary, #2b6cb0);
                    border: 1px solid var(--color-bg-secondary, #bee3f8);
                    padding: 4px 8px;
                    border-radius: 9999px;
                    font-size: var(--font-size-2xs, 0.75rem);
                    font-weight: 600;
                    font-family: var(--font-mono, monospace);
                    display: inline-block;
                    margin-right: 6px;
                    margin-bottom: 6px;
                }
                .mfe-badge-green {
                    background-color: #f0fff4;
                    color: #38a169;
                    border-color: #c6f6d5;
                }
                
                /* Config Accordion styling */
                .mfe-config-container {
                    background: var(--color-bg-surface, white);
                    border-radius: 12px;
                    border: 1px solid var(--color-bg-secondary, #edf2f7);
                    box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1);
                    overflow: hidden;
                    margin-bottom: 24px;
                }
                .mfe-config-header {
                    background: linear-gradient(135deg, var(--color-bg-secondary, #2d3748) 0%, var(--color-bg-surface, #4a5568) 100%);
                    color: var(--color-text-primary, white);
                    padding: 20px;
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    border-bottom: 1px solid var(--color-bg-secondary, transparent);
                }
                .mfe-config-header h2 {
                    font-family: var(--font-display, inherit);
                    font-size: var(--font-size-lg, 1.25rem);
                    margin: 0;
                    font-weight: 700;
                }
                
                .mfe-section {
                    border-bottom: 1px solid var(--color-bg-secondary, #edf2f7);
                }
                .mfe-section-header {
                    background: var(--color-bg-surface, #f7fafc);
                    color: var(--color-text-primary, inherit);
                    padding: 15px 20px;
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    cursor: pointer;
                    font-weight: 600;
                }
                .mfe-section-header:hover {
                    background: var(--color-bg-secondary, #edf2f7);
                }
                .mfe-section-content {
                    padding: 0;
                    display: block;
                }
                
                /* Table styling */
                .mfe-table {
                    width: 100%;
                    border-collapse: collapse;
                    text-align: left;
                }
                .mfe-table th {
                    background: var(--color-bg-secondary, #edf2f7);
                    color: var(--color-text-secondary, #4a5568);
                    padding: 10px 20px;
                    font-size: var(--font-size-xs, 0.85rem);
                    text-transform: uppercase;
                    font-weight: 700;
                    border-bottom: 2px solid var(--color-bg-primary, #cbd5e0);
                }
                .mfe-table td {
                    padding: 12px 20px;
                    border-bottom: 1px solid var(--color-bg-secondary, #edf2f7);
                    color: var(--color-text-primary, inherit);
                    font-size: var(--font-size-sm, 0.95rem);
                    vertical-align: middle;
                }
                .mfe-editable {
                    cursor: pointer;
                    padding: 4px 8px;
                    border-radius: 4px;
                    font-family: var(--font-mono, monospace);
                    background: var(--color-bg-primary, #f7fafc);
                    border: 1px solid var(--color-bg-secondary, #e2e8f0);
                    color: var(--color-text-primary, inherit);
                    word-break: break-all;
                }
                .mfe-editable:hover {
                    background: var(--color-bg-secondary, #edf2f7);
                    border-color: var(--color-bg-primary, #cbd5e0);
                }
                .mfe-mono {
                    font-family: var(--font-mono, monospace);
                    font-weight: 600;
                }
                .mfe-actions {
                    text-align: right;
                }
                .mfe-btn-circle {
                    width: 32px;
                    height: 32px;
                    border-radius: 50%;
                    display: inline-flex;
                    align-items: center;
                    justify-content: center;
                    border: 1px solid var(--color-bg-secondary, #e2e8f0);
                    background: var(--color-bg-surface, white);
                    cursor: pointer;
                    color: var(--color-text-primary, #4a5568);
                    transition: all 0.2s;
                }
                .mfe-btn-circle:hover {
                    background: var(--color-bg-secondary, #edf2f7);
                    color: var(--color-accent-primary, #2b6cb0);
                    border-color: var(--color-bg-primary, #cbd5e0);
                }
                
                /* Custom Vanilla CSS Modal */
                .mfe-modal {
                    position: fixed;
                    top: 0;
                    left: 0;
                    width: 100%;
                    height: 100%;
                    background: rgba(0,0,0,0.5);
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    z-index: 9999;
                    opacity: 0;
                    pointer-events: none;
                    transition: opacity 0.2s;
                }
                .mfe-modal.show {
                    opacity: 1;
                    pointer-events: auto;
                }
                .mfe-modal-content {
                    background: var(--color-bg-surface, white);
                    color: var(--color-text-primary, inherit);
                    border: 1px solid var(--color-bg-secondary, #edf2f7);
                    border-radius: 12px;
                    width: 90%;
                    max-width: 500px;
                    box-shadow: 0 20px 25px -5px rgba(0,0,0,0.1);
                    overflow: hidden;
                    transform: translateY(-20px);
                    transition: transform 0.2s;
                }
                .mfe-modal.show .mfe-modal-content {
                    transform: translateY(0);
                }
                .mfe-modal-header {
                    padding: 16px 20px;
                    color: white;
                    font-weight: 700;
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                }
                .mfe-modal-header-add { background: #6B0DF2; }
                .mfe-modal-header-edit { background: var(--color-bg-secondary, #2d3748); }
                
                .mfe-modal-body {
                    padding: 20px;
                }
                .mfe-modal-footer {
                    padding: 16px 20px;
                    background: var(--color-bg-primary, #f7fafc);
                    display: flex;
                    justify-content: flex-end;
                    gap: 10px;
                    border-top: 1px solid var(--color-bg-secondary, #e2e8f0);
                }
                .mfe-form-group {
                    margin-bottom: 16px;
                }
                .mfe-form-group label {
                    display: block;
                    font-weight: 600;
                    font-size: var(--font-size-sm, 0.9rem);
                    margin-bottom: 6px;
                    color: var(--color-text-secondary, #4a5568);
                }
                .mfe-form-input {
                    width: 100%;
                    padding: 10px 12px;
                    border: 1px solid var(--color-bg-secondary, #cbd5e0);
                    background: var(--color-bg-primary, white);
                    color: var(--color-text-primary, inherit);
                    border-radius: 6px;
                    font-family: inherit;
                    font-size: var(--font-size-sm, 0.9rem);
                    box-sizing: border-box;
                }
                .mfe-form-input:focus {
                    outline: none;
                    border-color: #6B0DF2;
                    box-shadow: 0 0 0 3px rgba(107, 13, 242, 0.15);
                }
                .mfe-close-btn {
                    cursor: pointer;
                    font-size: 1.5rem;
                    line-height: 1;
                }
            </style>
            
            <div class="mfe-container">
                <!-- Header -->
                <div class="mfe-header">
                    <div class="mfe-title">
                        <h1>🔔 Notification Server Manager</h1>
                        <p>OpenMFE dynamic alerting engine and log listener dashboard</p>
                    </div>
                    <div class="mfe-btn-group">
                        <button class="mfe-btn mfe-btn-action" id="mfe-reload-btn">
                            <i class="fa fa-refresh"></i> Reload Senders
                        </button>
                    </div>
                </div>
                
                <!-- Status Row -->
                <div class="mfe-row">
                    <div class="mfe-col-4">
                        <div class="mfe-card" style="height: 100%; box-sizing: border-box;">
                            <div class="mfe-card-title">Service Status</div>
                            <div class="mfe-status-indicator">
                                <span class="mfe-dot mfe-dot-inactive" id="mfe-status-dot"></span>
                                <span id="mfe-status-text">Offline</span>
                            </div>
                            <div style="color:#718096; font-size:0.85rem;" id="mfe-status-details">
                                Loading status...
                            </div>
                        </div>
                    </div>
                    <div class="mfe-col-8">
                        <div class="mfe-card" style="height: 100%; box-sizing: border-box;">
                            <div class="mfe-card-title">Active Notification Senders</div>
                            <div id="mfe-senders-container" style="max-height:100px; overflow-y:auto; display:flex; flex-wrap:wrap; gap:8px;">
                                Loading senders...
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Send Test Notification -->
                <div class="mfe-card" style="margin-bottom: 24px;">
                    <div class="mfe-card-title">Test Delivery Sandbox</div>
                    <form id="mfe-test-notif-form" style="display:flex; gap:15px; flex-wrap:wrap; align-items:flex-end;">
                        <div class="mfe-form-group" style="flex:1; min-width:120px; margin-bottom:0;">
                            <label>Level</label>
                            <select id="test-level" class="mfe-form-input">
                                <option value="INFO">INFO</option>
                                <option value="WARNING">WARNING</option>
                                <option value="ERROR">ERROR</option>
                                <option value="DEBUG">DEBUG</option>
                            </select>
                        </div>
                        <div class="mfe-form-group" style="flex:2; min-width:200px; margin-bottom:0;">
                            <label>Title</label>
                            <input type="text" id="test-title" class="mfe-form-input" placeholder="e.g. System Alert">
                        </div>
                        <div class="mfe-form-group" style="flex:4; min-width:300px; margin-bottom:0;">
                            <label>Message</label>
                            <input type="text" id="test-message" class="mfe-form-input" required placeholder="Type test notification message...">
                        </div>
                        <button type="submit" class="mfe-btn mfe-btn-primary" style="height:42px;">
                            <i class="fa fa-paper-plane"></i> Send Test
                        </button>
                    </form>
                </div>
                
                <!-- Configurations Accordion -->
                <div class="mfe-config-container">
                    <div class="mfe-config-header">
                        <h2><i class="fa fa-bell-o"></i> Alerting Providers</h2>
                        <button class="mfe-btn mfe-btn-secondary" style="padding: 6px 12px; font-size:0.8rem;" id="mfe-add-provider-btn">
                            <i class="fa fa-plus"></i> Add Provider
                        </button>
                    </div>
                    <div id="mfe-accordion-list">
                        <div style="padding:40px; text-align:center; color:#718096;">
                            <i class="fa fa-spinner fa-spin fa-2x"></i><br>Loading alerting providers...
                        </div>
                    </div>
                </div>
            </div>

            <!-- Add Provider Modal -->
            <div class="mfe-modal" id="mfe-add-modal">
                <div class="mfe-modal-content">
                    <div class="mfe-modal-header mfe-modal-header-add">
                        <span>Register Alerting Provider</span>
                        <span class="mfe-close-btn" onclick="document.getElementById('mfe-add-modal').classList.remove('show')">&times;</span>
                    </div>
                    <form id="mfe-add-form">
                        <div class="mfe-modal-body">
                            <div class="mfe-form-group">
                                <label>Provider Type / Driver</label>
                                <select id="add-type" class="mfe-form-input" required>
                                    <option value="" disabled selected>Select driver type...</option>
                                </select>
                            </div>
                            <div class="mfe-form-group">
                                <label>Unique Tag Name</label>
                                <input type="text" id="add-tag" class="mfe-form-input" required placeholder="e.g. telegram_production">
                            </div>
                        </div>
                        <div class="mfe-modal-footer">
                            <button type="button" class="mfe-btn mfe-btn-secondary" onclick="document.getElementById('mfe-add-modal').classList.remove('show')">Cancel</button>
                            <button type="submit" class="mfe-btn mfe-btn-primary">Initialize Provider</button>
                        </div>
                    </form>
                </div>
            </div>

            <!-- Edit Modal -->
            <div class="mfe-modal" id="mfe-edit-modal">
                <div class="mfe-modal-content">
                    <div class="mfe-modal-header mfe-modal-header-edit">
                        <span>Edit Setting</span>
                        <span class="mfe-close-btn" onclick="document.getElementById('mfe-edit-modal').classList.remove('show')">&times;</span>
                    </div>
                    <form id="mfe-edit-form">
                        <div class="mfe-modal-body">
                            <input type="hidden" id="edit-platform">
                            <input type="hidden" id="edit-key">
                            <div style="margin-bottom: 12px; font-size: 0.85rem; color: #4a5568;">
                                <strong>Target Parameter:</strong> <span id="edit-identifier" class="mfe-mono">provider -> key</span>
                            </div>
                            <div class="mfe-form-group">
                                <label>Value</label>
                                <textarea id="edit-value" class="mfe-form-input" required rows="4"></textarea>
                            </div>
                        </div>
                        <div class="mfe-modal-footer">
                            <button type="button" class="mfe-btn mfe-btn-secondary" onclick="document.getElementById('mfe-edit-modal').classList.remove('show')">Cancel</button>
                            <button type="submit" class="mfe-btn mfe-btn-primary">Save Changes</button>
                        </div>
                    </form>
                </div>
            </div>
        `;

        this.bindEvents();
    }

    bindEvents() {
        this.querySelector('#mfe-reload-btn').addEventListener('click', () => this.reloadSenders());
        this.querySelector('#mfe-add-provider-btn').addEventListener('click', () => {
            this.querySelector('#mfe-add-form').reset();
            this.querySelector('#mfe-add-modal').classList.add('show');
        });

        this.querySelector('#mfe-test-notif-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.sendTestNotification(
                this.querySelector('#test-level').value,
                this.querySelector('#test-title').value,
                this.querySelector('#test-message').value
            );
        });

        this.querySelector('#mfe-add-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitAddProvider(
                this.querySelector('#add-tag').value,
                this.querySelector('#add-type').value
            );
            this.querySelector('#mfe-add-modal').classList.remove('show');
        });

        this.querySelector('#mfe-edit-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitEditConfig(
                this.querySelector('#edit-platform').value,
                this.querySelector('#edit-key').value,
                this.querySelector('#edit-value').value
            );
            this.querySelector('#mfe-edit-modal').classList.remove('show');
        });
    }

    async loadAll() {
        await Promise.all([this.loadStatus(), this.loadNotifiers(), this.loadAlertingConfig(), this.loadSupportedTypes()]);
    }

    async loadStatus() {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/status`);
            if (res.ok) {
                this.statusData = await res.json();
                this.displayStatus();
            } else {
                throw new Error('Unreachable status');
            }
        } catch (err) {
            console.error('Failed to load status:', err);
            this.statusData.healthy = false;
            this.statusData.status = 'Error';
            this.displayStatus();
        }
    }

    displayStatus() {
        const dot = this.querySelector('#mfe-status-dot');
        const text = this.querySelector('#mfe-status-text');
        const details = this.querySelector('#mfe-status-details');

        if (dot && text) {
            if (this.statusData.healthy) {
                dot.className = 'mfe-dot mfe-dot-active';
                text.innerText = ' Operational';
            } else {
                dot.className = 'mfe-dot mfe-dot-inactive';
                text.innerText = ` ${this.statusData.status}`;
            }
        }

        const date = new Date(this.statusData.timestamp * 1000).toLocaleString();
        const activeCount = this.statusData.details ? (this.statusData.details.active_notifiers || '0') : '0';
        details.innerHTML = `
            <div><strong>Version:</strong> ${this.escapeHTML(this.statusData.version || 'N/A')}</div>
            <div><strong>Active Notifiers:</strong> ${this.escapeHTML(activeCount)}</div>
            <div><strong>Last Check:</strong> ${date}</div>
        `;
    }

    async loadNotifiers() {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/notifiers/list`);
            if (res.ok) {
                const data = await res.json();
                this.notifiersList = data.notifiers || [];
                this.displayNotifiers();
            }
        } catch (err) {
            console.error('Failed to load active notifiers list:', err);
            this.querySelector('#mfe-senders-container').innerHTML = `<p style="color:#e53e3e; font-size:0.85rem; margin:0;">Failed to fetch active senders.</p>`;
        }
    }

    displayNotifiers() {
        const container = this.querySelector('#mfe-senders-container');
        if (this.notifiersList.length > 0) {
            container.innerHTML = this.notifiersList.map(n => `
                <span class="mfe-badge mfe-badge-green"><i class="fa fa-check-circle"></i> ${this.escapeHTML(n.name)} <span style="opacity:0.7; font-size:0.7rem;">(${this.escapeHTML(n.type)})</span></span>
            `).join('');
        } else {
            container.innerHTML = `<p style="color:#718096; font-size:0.85rem; margin:0;">No active dispatch workers registered in background pool.</p>`;
        }
    }

    async loadSupportedTypes() {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/config/alerting/types`);
            if (res.ok) {
                const data = await res.json();
                this.supportedTypes = data.types || [];
                const select = this.querySelector('#add-type');
                select.innerHTML = '<option value="" disabled selected>Select driver type...</option>' + 
                    this.supportedTypes.map(t => `<option value="${this.escapeAttribute(t)}">${this.escapeHTML(t)}</option>`).join('');
            }
        } catch (err) {
            console.error('Failed to load supported types:', err);
        }
    }

    async loadAlertingConfig() {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/config/alerting/get`);
            if (res.ok) {
                const data = await res.json();
                this.alertingConfig = data.config || {};
                this.displayAlertingConfig();
            } else {
                throw new Error('Unreachable alerting config');
            }
        } catch (err) {
            console.error('Failed to load alerting config:', err);
            this.querySelector('#mfe-accordion-list').innerHTML = `
                <div style="padding:40px; text-align:center; color:#e53e3e;">
                    <i class="fa fa-exclamation-triangle fa-2x"></i><br>Failed to retrieve config parameters from notif-server: ${this.escapeHTML(err.message)}
                </div>
            `;
        }
    }

    stringifyValue(value) {
        if (value === null || value === undefined) {
            return '';
        }
        return String(value);
    }

    escapeHTML(value) {
        return this.stringifyValue(value).replace(/[&<>"']/g, char => ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#39;'
        }[char]));
    }

    escapeAttribute(value) {
        return this.escapeHTML(value);
    }

    displayAlertingConfig() {
        const container = this.querySelector('#mfe-accordion-list');
        container.innerHTML = '';

        const sections = Object.keys(this.alertingConfig);
        if (sections.length === 0) {
            container.innerHTML = `
                <div style="padding:40px; text-align:center; color:#718096;">
                    <i class="fa fa-bell-slash-o fa-2x"></i><br>No alerting providers initialized. Use the "Add Provider" button to start.
                </div>
            `;
            return;
        }

        sections.forEach((section, index) => {
            const secData = this.alertingConfig[section] || {};
            const settings = secData.settings || {};
            const sectionDiv = document.createElement('div');
            sectionDiv.className = 'mfe-section';

            const keys = Object.keys(settings);
            const contentId = `mfe-collapse-${index}`;
            const sectionLabel = this.escapeHTML(section);
            const typeLabel = this.escapeHTML(settings['TYPE'] || 'UNKNOWN');
            
            sectionDiv.innerHTML = `
                <div class="mfe-section-header" data-target="${contentId}">
                    <span><i class="fa fa-folder-open-o" style="color:#6B0DF2; margin-right:8px;"></i> <strong>${sectionLabel}</strong> <span style="font-size:0.75rem; background:#ebf8ff; color:#2b6cb0; padding:2px 6px; border-radius:10px; margin-left:8px;">${typeLabel}</span></span>
                    <i class="fa fa-chevron-down" style="color:#718096; font-size:0.75rem;"></i>
                </div>
                <div class="mfe-section-content" id="${contentId}">
                    <div style="overflow-x:auto;">
                        <table class="mfe-table">
                            <thead>
                                <tr>
                                    <th style="width:30%;">Key</th>
                                    <th style="width:55%;">Value</th>
                                    <th style="width:15%; text-align:right;">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${keys.map(key => {
                                    const value = this.stringifyValue(settings[key]);
                                    return `
                                    <tr>
                                        <td class="mfe-mono">${this.escapeHTML(key)}</td>
                                        <td>
                                            <div class="mfe-editable" data-platform="${this.escapeAttribute(section)}" data-key="${this.escapeAttribute(key)}">${this.escapeHTML(value)}</div>
                                        </td>
                                        <td class="mfe-actions">
                                            <button class="mfe-btn-circle mfe-edit-btn" data-platform="${this.escapeAttribute(section)}" data-key="${this.escapeAttribute(key)}" data-val="${this.escapeAttribute(value)}" title="Edit setting">
                                                <i class="fa fa-pencil"></i>
                                            </button>
                                        </td>
                                    </tr>
                                `}).join('')}
                            </tbody>
                        </table>
                        <div style="padding: 12px 20px; text-align: right; background: #fafafa; border-top: 1px solid #edf2f7;">
                            <button class="mfe-btn mfe-btn-secondary mfe-delete-provider-btn" data-platform="${this.escapeAttribute(section)}" style="padding: 6px 12px; font-size:0.8rem; background: #fff5f5; color: #e53e3e; border: 1px solid #fed7d7;">
                                <i class="fa fa-trash"></i> Delete Provider
                            </button>
                        </div>
                    </div>
                </div>
            `;

            container.appendChild(sectionDiv);
        });

        // Bind accordion collapses
        this.querySelectorAll('.mfe-section-header').forEach(header => {
            header.addEventListener('click', () => {
                const targetId = header.dataset.target;
                const content = this.querySelector(`#${targetId}`);
                if (content.style.display === 'none') {
                    content.style.display = 'block';
                    header.querySelector('.fa-chevron-down').style.transform = 'rotate(0deg)';
                } else {
                    content.style.display = 'none';
                    header.querySelector('.fa-chevron-down').style.transform = 'rotate(-90deg)';
                }
            });
        });

        // Bind inline edit clicks and pencil buttons
        const triggerEdit = (platform, key, val) => {
            this.querySelector('#edit-platform').value = platform;
            this.querySelector('#edit-key').value = key;
            this.querySelector('#edit-value').value = val;
            this.querySelector('#edit-identifier').innerText = `${platform} -> ${key}`;
            this.querySelector('#mfe-edit-modal').classList.add('show');
        };

        this.querySelectorAll('.mfe-editable').forEach(el => {
            el.addEventListener('click', () => triggerEdit(el.dataset.platform, el.dataset.key, el.innerText));
        });

        this.querySelectorAll('.mfe-edit-btn').forEach(btn => {
            btn.addEventListener('click', () => triggerEdit(btn.dataset.platform, btn.dataset.key, btn.dataset.val));
        });

        // Bind delete provider buttons
        this.querySelectorAll('.mfe-delete-provider-btn').forEach(btn => {
            btn.addEventListener('click', () => this.deleteProvider(btn.dataset.platform));
        });
    }

    async sendTestNotification(level, title, message) {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/notif/test`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ level, title, message })
            });

            const data = await res.json();
            if (res.ok && data.success) {
                alert("Test notification successfully queued for dispatch!");
                this.querySelector('#test-message').value = '';
            } else {
                alert(`Failed to send notification: ${data.message || 'Unknown error'}`);
            }
        } catch (err) {
            alert(`Failed to trigger test delivery: ${err.message}`);
        }
    }

    async submitAddProvider(tag, type) {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/config/alerting/add`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tag, type })
            });

            const data = await res.json();
            if (res.ok && data.success) {
                alert(`Alerting provider '${tag}' initialized successfully.`);
                await this.loadAll();
            } else {
                alert(`Failed to initialize provider: ${data.message || 'Unknown error'}`);
            }
        } catch (err) {
            alert(`Failed to create provider: ${err.message}`);
        }
    }

    async submitEditConfig(platform, key, value) {
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/config/alerting/set`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ platform, key, value })
            });

            const data = await res.json();
            if (res.ok && data.success) {
                await this.loadAll();
            } else {
                alert(`Error saving configuration: ${data.message || 'Unknown error'}`);
            }
        } catch (err) {
            alert(`Failed to save configuration: ${err.message}`);
        }
    }

    async deleteProvider(platform) {
        if (!confirm(`Are you sure you want to permanently delete the alerting provider '${platform}'? This cannot be undone.`)) {
            return;
        }
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/config/alerting/remove`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tag: platform })
            });

            const data = await res.json();
            if (res.ok && data.success) {
                alert(`Provider '${platform}' removed.`);
                await this.loadAll();
            } else {
                alert(`Failed to delete provider: ${data.message || 'Unknown error'}`);
            }
        } catch (err) {
            alert(`Delete provider failed: ${err.message}`);
        }
    }

    async reloadSenders() {
        if (!confirm("Are you sure you want to reload all sender configurations? This will apply the baseline or latest memory-mapped updates.")) {
            return;
        }
        try {
            const res = await fetch(`${this.baseUrl}/api/v1/config/reload`, { method: 'POST' });
            const data = await res.json();
            if (res.ok && data.success) {
                alert("Sender engines reloaded successfully.");
                await this.loadAll();
            } else {
                alert(`Reload failed: ${data.message || 'Unknown error'}`);
            }
        } catch (err) {
            alert(`Reload failed: ${err.message}`);
        }
    }
}

customElements.define('notif-server-mfe', NotifServerMFE);
