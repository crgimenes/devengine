// EAV Records List - Infinite Scroll with Intersection Observer
document.addEventListener('DOMContentLoaded', () => {
    const container = document.getElementById('records-container');
    const sentinel = document.getElementById('sentinel');

    if (!container || !sentinel) {
        return; // No infinite scroll needed
    }

    let entityID = container.dataset.entityId;
    let offset = parseInt(container.dataset.offset) || 0;
    let hasMore = container.dataset.hasMore === 'true';
    let loading = false;

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
                <span class="visually-hidden">Carregando...</span>
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
                sentinel.style.display = 'none';
                observer.disconnect();
            }
        } catch (error) {
            console.error('Error loading more records:', error);
            sentinel.innerHTML = `
                <div class="text-danger">
                    <i class="bi bi-exclamation-circle"></i> Erro ao carregar registros
                </div>
            `;
        } finally {
            loading = false;
        }
    }

    function createRecordCard(recordWithValues, entityID) {
        const col = document.createElement('div');
        col.className = 'col-12 col-md-6 col-lg-4';

        const record = recordWithValues.Record;
        const values = recordWithValues.Values;

        // Status badge
        const statusBadge = record.Status === 'active'
            ? '<span class="badge bg-success">Ativo</span>'
            : '<span class="badge bg-warning">Rascunho</span>';

        const refIDBadge = `<span class="badge bg-secondary font-monospace">${record.ReferenceID.substring(0, 8)}</span>`;

        // Values display
        let valuesHTML = '';
        for (const [key, value] of Object.entries(values)) {
            let displayValue = value;
            if (typeof value === 'boolean') {
                displayValue = value ? 'Sim' : 'Não';
            }
            valuesHTML += `
                <div class="mb-1">
                    <small class="text-muted">${key}:</small><br>
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
                        <small class="text-muted d-block">Criado: ${record.CreatedAt}</small>
                        <small class="text-muted d-block">Rev: ${record.Rev}</small>
                    </div>
                    <div class="btn-group w-100" role="group">
                        <a href="/tools/database-schema/eav/${entityID}/records/${record.ReferenceID}/edit" 
                           class="btn btn-sm btn-outline-primary">
                            Editar
                        </a>
                        <form method="POST" 
                              action="/tools/database-schema/eav/${entityID}/records/${record.ReferenceID}/delete" 
                              style="display: inline;"
                              class="delete-record-form">
                            <button type="submit" class="btn btn-sm btn-outline-danger">
                                Excluir
                            </button>
                        </form>
                    </div>
                </div>
            </div>
        `;

        // Add delete confirmation to the newly created form
        const deleteForm = col.querySelector('.delete-record-form');
        deleteForm.addEventListener('submit', event => {
            if (!confirm('Tem certeza que deseja excluir este registro?')) {
                event.preventDefault();
            }
        });

        return col;
    }

    // Delete confirmation for existing records
    const deleteForms = document.querySelectorAll('.delete-record-form');
    deleteForms.forEach(form => {
        form.addEventListener('submit', event => {
            if (!confirm('Tem certeza que deseja excluir este registro?')) {
                event.preventDefault();
            }
        });
    });
});
