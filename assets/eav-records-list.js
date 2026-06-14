// EAV Records List - Infinite Scroll with Intersection Observer
document.addEventListener('DOMContentLoaded', () => {
    const container = document.getElementById('records-container');
    const sentinel = document.getElementById('sentinel');

    // Delete confirmation must run even when there is no infinite scroll
    // (fewer records than a page means no sentinel), so wire it up first.
    document.querySelectorAll('.delete-record-form').forEach(form => {
        form.addEventListener('submit', event => {
            if (!confirm('Are you sure you want to delete this record?')) {
                event.preventDefault();
            }
        });
    });

    if (!container || !sentinel) {
        return; // No infinite scroll needed
    }

    let entityID = container.dataset.entityId;
    let offset = parseInt(container.dataset.offset) || 0;
    let hasMore = container.dataset.hasMore === 'true';
    let loading = false;

    let attributes = [];
    try {
        attributes = JSON.parse(container.dataset.attributes || '[]');
    } catch (e) {
        attributes = [];
    }

    // Intersection Observer for infinite scroll
    const observer = new IntersectionObserver((entries) => {
        const entry = entries[0];

        if (entry.isIntersecting && hasMore && !loading) {
            loadMore();
        }
    }, {
        root: null,
        rootMargin: '100px', // Trigger 100px before sentinel is visible
        threshold: 0.1
    });

    observer.observe(sentinel);

    async function loadMore() {
        loading = true;
        sentinel.innerHTML = `
            <div class="spinner-border text-primary" role="status">
                <span class="visually-hidden">Loading...</span>
            </div>
        `;

        try {
            const response = await fetch(`/tools/database-schema/eav/${entityID}/records/api?offset=${offset + 100}`);

            if (!response.ok) {
                throw new Error('Failed to load records');
            }

            const data = await response.json();

            // Render new cards
            data.records.forEach(recordWithValues => {
                const card = createRecordCard(recordWithValues, entityID);
                container.appendChild(card);
            });

            // Update state
            offset = data.offset;
            hasMore = data.hasMore;
            container.dataset.offset = offset;
            container.dataset.hasMore = hasMore;

            // Hide sentinel if no more records
            if (!hasMore) {
                sentinel.classList.add('d-none');
                observer.disconnect();
            }
        } catch (error) {
            console.error('Error loading more records:', error);
            sentinel.innerHTML = `
                <div class="text-danger">
                    <i class="bi bi-exclamation-circle"></i> Could not load records
                </div>
            `;
        } finally {
            loading = false;
        }
    }

    // Format an ISO datetime string as dd/mm/yyyy HH:MM. Leaves anything that
    // is not a full ISO datetime untouched.
    function formatDatetime(value) {
        if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(value)) {
            return value;
        }
        const d = new Date(value);
        if (isNaN(d.getTime())) {
            return value;
        }
        const pad = n => String(n).padStart(2, '0');
        return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
    }

    function createRecordCard(recordWithValues, entityID) {
        const col = document.createElement('div');
        col.className = 'col-12 col-md-6 col-lg-4';

        const record = recordWithValues.Record;
        const values = recordWithValues.Values;

        // Status badge
        const statusBadge = record.Status === 'active'
            ? '<span class="badge bg-success">Active</span>'
            : '<span class="badge bg-warning">Draft</span>';

        const refIDBadge = `<span class="badge bg-secondary font-monospace">${record.ReferenceID.substring(0, 8)}</span>`;

        // Values display: iterate every attribute so cards keep a uniform height
        const attrList = attributes.length
            ? attributes
            : Object.keys(values).map(k => ({ machine_name: k, label: k }));
        let valuesHTML = '';
        for (const attr of attrList) {
            const value = values[attr.machine_name];
            let displayValue;
            if (value === undefined || value === null) {
                displayValue = '<span class="text-muted">—</span>';
            } else if (typeof value === 'boolean') {
                displayValue = value ? 'Yes' : 'No';
            } else {
                displayValue = formatDatetime(value);
            }
            valuesHTML += `
                <div class="mb-1">
                    <small class="text-muted">${attr.label}:</small><br>
                    <strong>${displayValue}</strong>
                </div>
            `;
        }

        col.innerHTML = `
            <div class="card h-100">
                <div class="card-body">
                    <div class="mb-2">
                        ${statusBadge}
                        ${refIDBadge}
                    </div>
                    <div class="mb-3">
                        ${valuesHTML}
                    </div>
                    <div class="mb-3">
                        <small class="text-muted d-block">Created: ${formatDatetime(record.CreatedAt)}</small>
                        <small class="text-muted d-block">Rev: ${record.Rev}</small>
                    </div>
                    <div class="btn-group w-100" role="group">
                        <a href="/tools/database-schema/eav/${entityID}/records/${record.ReferenceID}/edit" 
                           class="btn btn-sm btn-outline-primary">
                            Edit
                        </a>
                        <form method="POST" 
                              action="/tools/database-schema/eav/${entityID}/records/${record.ReferenceID}/delete" 
                              style="display: inline;"
                              class="delete-record-form">
                            <button type="submit" class="btn btn-sm btn-outline-danger">
                                Delete
                            </button>
                        </form>
                    </div>
                </div>
            </div>
        `;

        // Add delete confirmation to the newly created form
        const deleteForm = col.querySelector('.delete-record-form');
        deleteForm.addEventListener('submit', event => {
            if (!confirm('Are you sure you want to delete this record?')) {
                event.preventDefault();
            }
        });

        return col;
    }
});
