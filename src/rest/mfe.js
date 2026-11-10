// =============================================================================
// ESSENTIAL PROCESS:
// OpenMFE web component for notif-server, encapsulating dynamic alerting engine,
// provider management, and test notification dispatching.
//
// DATA FLOW:
// 1. Input: Status and provider configurations fetched via HTTP from /api/v1/ endpoints.
// 2. Logic: Renders responsive dashboard, interactive tables, test sandbox, and modals.
// 3. Output: Dispatches HTTP REST mutations to notif-server daemon.
//
// KEY PARAMETERS:
// - NotifServerMFE: Standard OpenMFE Web Component registered as notif-server-mfe.
// - Complies strictly with Bastien UI tokens and openmfe-components.css.
// =============================================================================

(function () {
    'use strict';

const BaseElementClass = (typeof window !== 'undefined' && window.BastienMFEElement)
    ? window.BastienMFEElement
    : class extends (typeof HTMLElement !== 'undefined' ? HTMLElement : class {}) {
        constructor() {
            super();
            this.baseUrl = '';
            this.pollTimers = [];
            this.dom = {};
            this.state = 'IDLE';
        }
        connectedCallback() {
            let origin = this.getAttribute('base-url') || (typeof window !== 'undefined' && window.location ? window.location.origin : '');
            if (origin.endsWith('/')) origin = origin.slice(0, -1);
            this.baseUrl = origin;
            this.render();
            this.cacheDom();
            this.bindEvents();
            this.onMount();
        }
        disconnectedCallback() {
            this.clearPollTimers();
            this.onUnmount();
        }
        render() {}
        cacheDom() {}
        bindEvents() {}
        onMount() {}
        onUnmount() {}
        registerPoll(cb, ms = 5000, imm = true) {
            if (imm) { try { cb(); } catch (e) { console.error(e); } }
            if (!this.pollTimers) this.pollTimers = [];
            const t = setInterval(cb, ms);
            this.pollTimers.push(t);
            return t;
        }
        clearPollTimers() {
            if (this.pollTimers) this.pollTimers.forEach(t => clearInterval(t));
            this.pollTimers = [];
        }
    };

class NotifServerMFE extends BaseElementClass {
    constructor() {
        super();
        this.statusData = { healthy: false, status: 'Offline', version: 'N/A', timestamp: 0, details: { active_notifiers: '0' } };
        this.notifiersList = [];
        this.alertingConfig = {};
        this.supportedTypes = [];
    }

    render() {
        this.innerHTML = `
            <div class="openmfe-container">
                <!-- Header Toolbar -->
                <div class="openmfe-toolbar">
                    <div class="openmfe-toolbar__title">
                        <i class="fa fa-bell-o" style="color: var(--color-accent-primary, #38bdf8);"></i>
                        <span>Notification & Alerting Manager</span>
                        <span class="openmfe-badge" id="mfe-status-badge">
                            <span class="openmfe-dot openmfe-dot--inactive" id="mfe-status-dot"></span>
                            <span id="mfe-status-text">Offline</span>
                        </span>
                    </div>
                    <div class="openmfe-toolbar__actions">
                        <button class="openmfe-btn openmfe-btn--sm openmfe-btn--outline" id="mfe-reload-btn" title="Reload active dispatch workers">
                            <i class="fa fa-refresh"></i> Reload Senders
                        </button>
                    </div>
                </div>
                
                <!-- Status Row -->
                <div class="openmfe-kpi-grid">
                    <!-- Service Status Card -->
                    <div class="openmfe-card">
                        <div class="openmfe-card__header">
                            <h4 class="openmfe-card__title"><i class="fa fa-heartbeat"></i> Service Status</h4>
                        </div>
                        <div class="openmfe-card__body">
                            <div style="display:flex; align-items:center; gap:8px; font-size:1.1rem; font-weight:700; margin-bottom:8px;">
                                <span class="openmfe-dot openmfe-dot--inactive" id="mfe-status-dot-inner"></span>
                                <span id="mfe-status-text-inner">Offline</span>
                            </div>
                            <div style="color:var(--color-text-muted, #94a3b8); font-size:0.8125rem; line-height:1.6;" id="mfe-status-details">
                                Loading status...
                            </div>
                        </div>
                    </div>

                    <!-- Active Notification Senders -->
                    <div class="openmfe-card">
                        <div class="openmfe-card__header">
                            <h4 class="openmfe-card__title"><i class="fa fa-paper-plane"></i> Active Dispatch Workers</h4>
                        </div>
                        <div class="openmfe-card__body">
                            <div id="mfe-senders-container" style="display:flex; flex-wrap:wrap; gap:6px; max-height:100px; overflow-y:auto;">
                                Loading senders...
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Sub-Navigation Tabs Bar (div[3]) -->
                <div class="openmfe-tabs-bar" id="notif-tabs-bar" role="tablist">
                    <button class="openmfe-tab-btn is-active" data-tab="operations" role="tab" type="button">
                        <i class="fa fa-bell"></i> Dispatch & Providers
                    </button>
                </div>

                <div id="notif-panels-host">
                    <div class="openmfe-panel is-active" id="panel-operations">
                        <!-- Send Test Notification Card -->
                        <div class="openmfe-card">
                            <div class="openmfe-card__header">
                                <h4 class="openmfe-card__title"><i class="fa fa-paper-plane-o"></i> Test Delivery Sandbox</h4>
                            </div>
                            <div class="openmfe-card__body">
                                <form id="mfe-test-notif-form" style="display:flex; gap:12px; flex-wrap:wrap; align-items:flex-end;">
                                    <div class="openmfe-form-group" style="flex:1; min-width:120px; margin-bottom:0;">
                                        <label class="openmfe-form-label">Level</label>
                                        <select id="test-level" class="openmfe-form-select">
                                            <option value="INFO">INFO</option>
                                            <option value="WARNING">WARNING</option>
                                            <option value="ERROR">ERROR</option>
                                            <option value="DEBUG">DEBUG</option>
                                        </select>
                                    </div>
                                    <div class="openmfe-form-group" style="flex:2; min-width:180px; margin-bottom:0;">
                                        <label class="openmfe-form-label">Title</label>
                                        <input type="text" id="test-title" class="openmfe-form-input" placeholder="e.g. System Alert">
                                    </div>
                                    <div class="openmfe-form-group" style="flex:3; min-width:260px; margin-bottom:0;">
                                        <label class="openmfe-form-label">Message</label>
                                        <input type="text" id="test-message" class="openmfe-form-input" required placeholder="Type test notification message...">
                                    </div>
                                    <button type="submit" class="openmfe-btn openmfe-btn--primary" style="height:38px;">
                                        <i class="fa fa-paper-plane"></i> Send Test
                                    </button>
                                </form>
                            </div>
                        </div>
                        
                        <!-- Configurations Accordion -->
                        <div class="openmfe-card" style="margin-top: var(--space-4, 16px);">
                            <div class="openmfe-card__header">
                                <h4 class="openmfe-card__title"><i class="fa fa-bell-o"></i> Alerting Providers</h4>
                                <button class="openmfe-btn openmfe-btn--sm openmfe-btn--outline" id="mfe-add-provider-btn">
                                    <i class="fa fa-plus"></i> Add Provider
                                </button>
                            </div>
                            <div class="openmfe-card__body" style="padding: 0;">
                                <div id="mfe-accordion-list">
                                    <div style="padding:40px; text-align:center; color:var(--color-text-muted, #94a3b8);">
                                        <i class="fa fa-spinner fa-spin fa-2x"></i><br><br>Loading alerting providers...
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Add Provider Modal -->
            <div class="openmfe-modal" id="mfe-add-modal">
                <div class="openmfe-modal-content">
                    <div class="openmfe-modal-header">
                        <span><i class="fa fa-plus-circle" style="color:var(--color-accent-primary, #38bdf8); margin-right:6px;"></i> Register Alerting Provider</span>
                        <span class="mfe-close-add-modal" style="cursor:pointer; font-size:1.25rem;">&times;</span>
                    </div>
                    <form id="mfe-add-form">
                        <div class="openmfe-modal-body">
                            <div class="openmfe-form-group">
                                <label class="openmfe-form-label">Provider Type / Driver</label>
                                <select id="add-type" class="openmfe-form-select" required>
                                    <option value="" disabled selected>Select driver type...</option>
                                </select>
                            </div>
                            <div class="openmfe-form-group">
                                <label class="openmfe-form-label">Unique Tag Name</label>
                                <input type="text" id="add-tag" class="openmfe-form-input" required placeholder="e.g. telegram_production">
                            </div>
                        </div>
                        <div class="openmfe-modal-footer">
                            <button type="button" class="openmfe-btn openmfe-btn--sm openmfe-btn--outline mfe-close-add-modal">Cancel</button>
                            <button type="submit" class="openmfe-btn openmfe-btn--sm openmfe-btn--primary">Initialize Provider</button>
                        </div>
                    </form>
                </div>
            </div>

            <!-- Edit Modal -->
            <div class="openmfe-modal" id="mfe-edit-modal">
                <div class="openmfe-modal-content">
                    <div class="openmfe-modal-header">
                        <span><i class="fa fa-pencil-square" style="color:var(--color-accent-primary, #38bdf8); margin-right:6px;"></i> Edit Setting</span>
                        <span class="mfe-close-edit-modal" style="cursor:pointer; font-size:1.25rem;">&times;</span>
                    </div>
                    <form id="mfe-edit-form">
                        <div class="openmfe-modal-body">
                            <input type="hidden" id="edit-platform">
                            <input type="hidden" id="edit-key">
                            <div style="margin-bottom: 12px; font-size: 0.8125rem; color: var(--color-text-muted, #94a3b8);">
                                <strong>Target Parameter:</strong> <span id="edit-identifier" style="font-family:var(--font-mono, monospace); color:var(--color-accent-primary, #38bdf8);">provider -> key</span>
                            </div>
                            <div class="openmfe-form-group">
                                <label class="openmfe-form-label">Value</label>
                                <textarea id="edit-value" class="openmfe-form-input" required rows="4"></textarea>
                            </div>
                        </div>
                        <div class="openmfe-modal-footer">
                            <button type="button" class="openmfe-btn openmfe-btn--sm openmfe-btn--outline mfe-close-edit-modal">Cancel</button>
                            <button type="submit" class="openmfe-btn openmfe-btn--sm openmfe-btn--primary">Save Changes</button>
                        </div>
                    </form>
                </div>
            </div>
        `;
    }

    cacheDom() {
        this.dom = {
            statusDot: this.querySelector('#mfe-status-dot'),
            statusDotInner: this.querySelector('#mfe-status-dot-inner'),
            statusText: this.querySelector('#mfe-status-text'),
            statusTextInner: this.querySelector('#mfe-status-text-inner'),
            statusDetails: this.querySelector('#mfe-status-details'),
            sendersContainer: this.querySelector('#mfe-senders-container'),
            accordionList: this.querySelector('#mfe-accordion-list'),
            btnReload: this.querySelector('#mfe-reload-btn'),
            btnAddProvider: this.querySelector('#mfe-add-provider-btn'),
            addModal: this.querySelector('#mfe-add-modal'),
            addForm: this.querySelector('#mfe-add-form'),
            editModal: this.querySelector('#mfe-edit-modal'),
            editForm: this.querySelector('#mfe-edit-form'),
            editPlatform: this.querySelector('#edit-platform'),
            editKey: this.querySelector('#edit-key'),
            editValue: this.querySelector('#edit-value'),
            editIdentifier: this.querySelector('#edit-identifier'),
            testForm: this.querySelector('#mfe-test-notif-form'),
            testLevel: this.querySelector('#test-level'),
            testTitle: this.querySelector('#test-title'),
            testMessage: this.querySelector('#test-message'),
            addTypeSelect: this.querySelector('#add-type'),
            addTagInput: this.querySelector('#add-tag'),
            tabsBar: this.querySelector('#notif-tabs-bar'),
            panelsHost: this.querySelector('#notif-panels-host')
        };
    }

    switchTab(tabName) {
        this.activeTab = tabName || 'operations';
        this.querySelectorAll('#notif-tabs-bar .openmfe-tab-btn').forEach(btn => {
            btn.classList.toggle('is-active', btn.getAttribute('data-tab') === this.activeTab);
        });
        this.querySelectorAll('#notif-panels-host > .openmfe-panel').forEach(panel => {
            panel.classList.toggle('is-active', panel.id === `panel-${this.activeTab}`);
        });
    }

    bindEvents() {
        this.querySelectorAll('#notif-tabs-bar .openmfe-tab-btn').forEach(btn => {
            btn.addEventListener('click', () => this.switchTab(btn.getAttribute('data-tab')));
        });
        this.querySelectorAll('.mfe-close-add-modal').forEach(el => {
            el.addEventListener('click', () => this.dom.addModal?.classList.remove('show'));
        });
        this.querySelectorAll('.mfe-close-edit-modal').forEach(el => {
            el.addEventListener('click', () => this.dom.editModal?.classList.remove('show'));
        });
        this.dom.btnReload?.addEventListener('click', () => this.reloadSenders());
        this.dom.btnAddProvider?.addEventListener('click', () => {
            this.dom.addForm?.reset();
            this.dom.addModal?.classList.add('show');
        });

        this.dom.testForm?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.sendTestNotification(
                this.dom.testLevel?.value,
                this.dom.testTitle?.value,
                this.dom.testMessage?.value
            );
        });

        this.dom.addForm?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitAddProvider(
                this.dom.addTagInput?.value,
                this.dom.addTypeSelect?.value
            );
            this.dom.addModal?.classList.remove('show');
        });

        this.dom.editForm?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitEditConfig(
                this.dom.editPlatform?.value,
                this.dom.editKey?.value,
                this.dom.editValue?.value
            );
            this.dom.editModal?.classList.remove('show');
        });
    }

    async onMount() {
        this.activeTab = this.activeTab || 'operations';
        await this.loadAll();
        this.registerPoll(() => this.loadStatus(), 5000, false);
    }

    async loadAll() {
        await Promise.all([this.loadStatus(), this.loadNotifiers(), this.loadAlertingConfig(), this.loadSupportedTypes(), this.loadContracts()]);
    }

    async loadContracts() {
        const contractsApi = typeof window !== 'undefined' ? window.BastienMFEContracts : null;
        if (!contractsApi || !this.dom.tabsBar || !this.dom.panelsHost) return;
        const envelope = await contractsApi.fetchContracts({
            primaryUrls: [`${this.baseUrl}/api/v1/contracts`],
            serviceName: 'notif_server'
        });
        contractsApi.mountContractTabs({
            rootEl: this,
            tabsBarEl: this.dom.tabsBar,
            panelsHostEl: this.dom.panelsHost,
            envelope,
            activeTab: this.activeTab,
            onSwitchTab: (tab) => this.switchTab(tab),
            onRetry: () => this.loadContracts()
        });
    }

    async loadStatus() {
        try {
            let url = `${this.baseUrl}/api/v1/status`;
            let res = await fetch(url).catch(() => null);
            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/status`).catch(() => null);
            }
            if (res && res.ok) {
                this.statusData = await res.json();
                this.displayStatus();
            } else {
                throw new Error('Unreachable status');
            }
        } catch (err) {
            this.statusData.healthy = false;
            this.statusData.status = 'Error';
            this.displayStatus();
        }
    }

    displayStatus() {
        const isOnline = this.statusData.healthy;
        const statusLabel = isOnline ? 'Operational' : ` ${this.statusData.status || 'Offline'}`;
        const dotClass = `openmfe-dot ${isOnline ? 'openmfe-dot--active' : 'openmfe-dot--inactive'}`;

        if (this.dom.statusDot) this.dom.statusDot.className = dotClass;
        if (this.dom.statusDotInner) this.dom.statusDotInner.className = dotClass;
        if (this.dom.statusText) this.dom.statusText.innerText = statusLabel;
        if (this.dom.statusTextInner) this.dom.statusTextInner.innerText = statusLabel;

        const date = this.statusData.timestamp ? new Date(this.statusData.timestamp * 1000).toLocaleString() : '—';
        const activeCount = this.statusData.details ? (this.statusData.details.active_notifiers || '0') : '0';
        if (this.dom.statusDetails) {
            this.dom.statusDetails.innerHTML = `
                <div><strong>Version:</strong> ${this.escapeHTML(this.statusData.version || 'N/A')}</div>
                <div><strong>Active Notifiers:</strong> ${this.escapeHTML(activeCount)}</div>
                <div><strong>Last Check:</strong> ${date}</div>
            `;
        }
    }

    async loadNotifiers() {
        try {
            let url = `${this.baseUrl}/api/v1/notifiers/list`;
            let res = await fetch(url).catch(() => null);
            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/notifiers/list`).catch(() => null);
            }
            if (res && res.ok) {
                const data = await res.json();
                this.notifiersList = data.notifiers || [];
                this.displayNotifiers();
            }
        } catch (err) {
            if (this.dom.sendersContainer) {
                this.dom.sendersContainer.innerHTML = `<p style="color:var(--color-danger, #fb7185); font-size:0.8125rem; margin:0;">Failed to fetch active senders.</p>`;
            }
        }
    }

    displayNotifiers() {
        if (!this.dom.sendersContainer) return;
        if (this.notifiersList.length > 0) {
            this.dom.sendersContainer.innerHTML = this.notifiersList.map(n => `
                <span class="openmfe-badge openmfe-badge--success"><i class="fa fa-check-circle"></i> ${this.escapeHTML(n.name)} <span style="opacity:0.7; font-size:0.7rem;">(${this.escapeHTML(n.type)})</span></span>
            `).join('');
        } else {
            this.dom.sendersContainer.innerHTML = `<p style="color:var(--color-text-muted, #94a3b8); font-size:0.8125rem; margin:0;">No active dispatch workers registered in background pool.</p>`;
        }
    }

    async loadSupportedTypes() {
        try {
            let url = `${this.baseUrl}/api/v1/config/alerting/types`;
            let res = await fetch(url).catch(() => null);
            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/config/alerting/types`).catch(() => null);
            }
            if (res && res.ok) {
                const data = await res.json();
                this.supportedTypes = data.types || [];
                if (this.dom.addTypeSelect) {
                    this.dom.addTypeSelect.innerHTML = '<option value="" disabled selected>Select driver type...</option>' +
                        this.supportedTypes.map(t => `<option value="${this.escapeAttribute(t)}">${this.escapeHTML(t)}</option>`).join('');
                }
            }
        } catch (err) {
            // silent fallback
        }
    }

    async loadAlertingConfig() {
        try {
            let url = `${this.baseUrl}/api/v1/config/alerting/get`;
            let res = await fetch(url).catch(() => null);
            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/config/alerting/get`).catch(() => null);
            }
            if (res && res.ok) {
                const data = await res.json();
                this.alertingConfig = data.config || {};
                this.displayAlertingConfig();
            } else {
                throw new Error('Unreachable alerting config');
            }
        } catch (err) {
            if (this.dom.accordionList) {
                this.dom.accordionList.innerHTML = `
                    <div style="padding:40px; text-align:center; color:var(--color-danger, #fb7185);">
                        <i class="fa fa-exclamation-triangle fa-2x"></i><br><br>Failed to retrieve config parameters from notif-server: ${this.escapeHTML(err.message)}
                    </div>
                `;
            }
        }
    }

    stringifyValue(value) {
        if (value === null || value === undefined) return '';
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
        if (!this.dom.accordionList) return;
        this.dom.accordionList.innerHTML = '';

        const sections = Object.keys(this.alertingConfig);
        if (sections.length === 0) {
            this.dom.accordionList.innerHTML = `
                <div style="padding:40px; text-align:center; color:var(--color-text-muted, #94a3b8);">
                    <i class="fa fa-bell-slash-o fa-2x"></i><br><br>No alerting providers initialized. Use the "Add Provider" button to start.
                </div>
            `;
            return;
        }

        sections.forEach((section, index) => {
            const secData = this.alertingConfig[section] || {};
            const settings = secData.settings || {};
            const sectionDiv = document.createElement('div');
            sectionDiv.style.borderBottom = '1px solid var(--color-border-subtle, rgba(255, 255, 255, 0.08))';

            const keys = Object.keys(settings);
            const contentId = `mfe-collapse-${index}`;
            const sectionLabel = this.escapeHTML(section);
            const typeLabel = this.escapeHTML(settings['TYPE'] || 'UNKNOWN');

            sectionDiv.innerHTML = `
                <div class="mfe-section-header" data-target="${contentId}" style="background:var(--color-bg-overlay, rgba(15,23,42,0.3)); color:var(--color-text-primary, #f8fafc); padding:12px 16px; display:flex; justify-content:space-between; align-items:center; cursor:pointer; font-weight:600; font-size:0.875rem;">
                    <span><i class="fa fa-folder-open-o" style="color:var(--color-accent-primary, #38bdf8); margin-right:8px;"></i> <strong>${sectionLabel}</strong> <span style="font-size:0.75rem; background:rgba(255,255,255,0.08); color:var(--color-accent-primary, #38bdf8); padding:2px 8px; border-radius:10px; margin-left:8px;">${typeLabel}</span></span>
                    <i class="fa fa-chevron-down" style="color:var(--color-text-muted, #94a3b8); font-size:0.75rem; transition:transform 0.2s;"></i>
                </div>
                <div class="mfe-section-content" id="${contentId}" style="display:block;">
                    <div style="overflow-x:auto;">
                        <table class="openmfe-table">
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
                                        <td style="font-family:var(--font-mono, monospace); font-weight:600; color:var(--color-text-primary, #f8fafc);">${this.escapeHTML(key)}</td>
                                        <td>
                                            <div class="mfe-editable" data-platform="${this.escapeAttribute(section)}" data-key="${this.escapeAttribute(key)}" style="cursor:pointer; padding:4px 8px; border-radius:4px; font-family:var(--font-mono, monospace); background:rgba(15,23,42,0.4); border:1px solid var(--color-border-subtle, rgba(255,255,255,0.06)); word-break:break-all;">${this.escapeHTML(value)}</div>
                                        </td>
                                        <td style="text-align:right;">
                                            <button class="openmfe-btn openmfe-btn--sm openmfe-btn--outline mfe-edit-btn" data-platform="${this.escapeAttribute(section)}" data-key="${this.escapeAttribute(key)}" data-val="${this.escapeAttribute(value)}" title="Edit setting">
                                                <i class="fa fa-pencil"></i>
                                            </button>
                                        </td>
                                    </tr>
                                    `;
                                }).join('')}
                            </tbody>
                        </table>
                        <div style="padding: 10px 16px; text-align: right; background: var(--color-bg-overlay, rgba(15,23,42,0.4)); border-top: 1px solid var(--color-border-subtle, rgba(255,255,255,0.06));">
                            <button class="openmfe-btn openmfe-btn--sm openmfe-btn--danger mfe-delete-provider-btn" data-platform="${this.escapeAttribute(section)}">
                                <i class="fa fa-trash"></i> Delete Provider
                            </button>
                        </div>
                    </div>
                </div>
            `;

            this.dom.accordionList.appendChild(sectionDiv);
        });

        // Bind accordion collapses
        this.dom.accordionList.querySelectorAll('.mfe-section-header').forEach(header => {
            header.addEventListener('click', () => {
                const targetId = header.dataset.target;
                const content = this.querySelector(`#${targetId}`);
                const icon = header.querySelector('.fa-chevron-down');
                if (content.style.display === 'none') {
                    content.style.display = 'block';
                    if (icon) icon.style.transform = 'rotate(0deg)';
                } else {
                    content.style.display = 'none';
                    if (icon) icon.style.transform = 'rotate(-90deg)';
                }
            });
        });

        // Bind inline edit clicks and pencil buttons
        const triggerEdit = (platform, key, val) => {
            if (this.dom.editPlatform) this.dom.editPlatform.value = platform;
            if (this.dom.editKey) this.dom.editKey.value = key;
            if (this.dom.editValue) this.dom.editValue.value = val;
            if (this.dom.editIdentifier) this.dom.editIdentifier.innerText = `${platform} -> ${key}`;
            this.dom.editModal?.classList.add('show');
        };

        this.dom.accordionList.querySelectorAll('.mfe-editable').forEach(el => {
            el.addEventListener('click', () => triggerEdit(el.dataset.platform, el.dataset.key, el.innerText));
        });

        this.dom.accordionList.querySelectorAll('.mfe-edit-btn').forEach(btn => {
            btn.addEventListener('click', () => triggerEdit(btn.dataset.platform, btn.dataset.key, btn.dataset.val));
        });

        // Bind delete provider buttons
        this.dom.accordionList.querySelectorAll('.mfe-delete-provider-btn').forEach(btn => {
            btn.addEventListener('click', () => this.deleteProvider(btn.dataset.platform));
        });
    }

    async sendTestNotification(level, title, message) {
        try {
            let url = `${this.baseUrl}/api/v1/notif/test`;
            let res = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ level, title, message })
            }).catch(() => null);

            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/notif/test`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ level, title, message })
                }).catch(() => null);
            }

            const data = res ? await res.json().catch(() => ({})) : {};
            if (res && res.ok && data.success) {
                alert("Test notification successfully queued for dispatch!");
                if (this.dom.testMessage) this.dom.testMessage.value = '';
            } else {
                alert(`Failed to send notification: ${data.message || 'Unknown error'}`);
            }
        } catch (err) {
            alert(`Failed to trigger test delivery: ${err.message}`);
        }
    }

    async submitAddProvider(tag, type) {
        try {
            let url = `${this.baseUrl}/api/v1/config/alerting/add`;
            let res = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tag, type })
            }).catch(() => null);

            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/config/alerting/add`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ tag, type })
                }).catch(() => null);
            }

            const data = res ? await res.json().catch(() => ({})) : {};
            if (res && res.ok && data.success) {
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
            let url = `${this.baseUrl}/api/v1/config/alerting/set`;
            let res = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ platform, key, value })
            }).catch(() => null);

            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/config/alerting/set`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ platform, key, value })
                }).catch(() => null);
            }

            const data = res ? await res.json().catch(() => ({})) : {};
            if (res && res.ok && data.success) {
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
            let url = `${this.baseUrl}/api/v1/config/alerting/remove`;
            let res = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tag: platform })
            }).catch(() => null);

            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/config/alerting/remove`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ tag: platform })
                }).catch(() => null);
            }

            const data = res ? await res.json().catch(() => ({})) : {};
            if (res && res.ok && data.success) {
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
            let url = `${this.baseUrl}/api/v1/config/reload`;
            let res = await fetch(url, { method: 'POST' }).catch(() => null);
            if ((!res || !res.ok) && typeof window !== 'undefined' && this.baseUrl !== window.location.origin) {
                res = await fetch(`${window.location.origin}/api/v1/config/reload`, { method: 'POST' }).catch(() => null);
            }

            const data = res ? await res.json().catch(() => ({})) : {};
            if (res && res.ok && data.success) {
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

// =============================================================================
// Registration Ritual
// =============================================================================

const registerMFE = (typeof window !== 'undefined' && window.BastienMFEElement && window.BastienMFEElement.register)
    ? window.BastienMFEElement.register
    : (tag, cls) => {
        if (typeof customElements !== 'undefined' && !customElements.get(tag)) {
            try { customElements.define(tag, cls); }
            catch (e) { class Alias extends cls {} customElements.define(tag, Alias); }
        }
    };

registerMFE('notif-server-mfe', NotifServerMFE);

})();
