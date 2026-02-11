const dropZone = document.getElementById('dropZone');
const fileInput = document.getElementById('fileInput');
const uploadBtn = document.getElementById('uploadBtn');
const historyBody = document.getElementById('historyBody');
const toast = document.getElementById('toast');

let selectedFile = null;

dropZone.addEventListener('click', () => fileInput.click());

dropZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    dropZone.classList.add('dragover');
});

dropZone.addEventListener('dragleave', () => {
    dropZone.classList.remove('dragover');
});

dropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropZone.classList.remove('dragover');

    const files = e.dataTransfer.files;
    if (files.length > 0) {
        selectedFile = files[0];
        updateDropZoneText();
    }
});

// Paste image anywhere on page => upload
document.addEventListener('paste', (e) => {
    const dt = e.clipboardData;
    if (!dt) return;

    let file = null;

    if (dt.items && dt.items.length) {
        for (const item of dt.items) {
            if (item.kind === 'file' && item.type?.startsWith('image/')) {
                file = item.getAsFile();
                break;
            }
        }
    }

    if (!file && dt.files && dt.files.length) {
        for (const f of dt.files) {
            if (f.type?.startsWith('image/')) {
                file = f;
                break;
            }
        }
    }

    if (!file) return;

    e.preventDefault();

    const ext = (file.type.split('/')[1] || 'png').toLowerCase();
    const safeName = `pasted-${Date.now()}.${ext}`;
    selectedFile = new File([file], safeName, { type: file.type });

    updateDropZoneText();

    if (!uploadBtn.disabled) uploadBtn.click();
});

fileInput.addEventListener('change', (e) => {
    if (e.target.files.length > 0) {
        selectedFile = e.target.files[0];
        updateDropZoneText();
    }
});

function updateDropZoneText() {
    const dropText = dropZone.querySelector('.drop-text');
    if (selectedFile) {
        dropText.textContent = selectedFile.name;
    }
}

uploadBtn.addEventListener('click', async () => {
    if (!selectedFile) {
        showToast('Выберите файл');
        return;
    }

    if (selectedFile.size > 100 * 1024 * 1024) {
        showToast('Файл слишком большой');
        return;
    }

    uploadBtn.disabled = true;
    uploadBtn.textContent = 'Загрузка...';

    const formData = new FormData();
    formData.append('file', selectedFile);

    try {
        const response = await fetch('/upload', {
            method: 'POST',
            body: formData
        });

        if (response.ok) {
            const data = await response.json();
            saveToHistory(selectedFile.name, data.link, data.deletion_link);
            loadHistory();

            selectedFile = null;
            fileInput.value = '';
            const dropText = dropZone.querySelector('.drop-text');
            dropText.textContent = 'Перетащите или выберите файл';

            showToast('Файл загружен');
        } else {
            showToast('Ошибка загрузки');
        }
    } catch (error) {
        showToast('Ошибка: ' + error.message);
    } finally {
        uploadBtn.disabled = false;
        uploadBtn.textContent = 'Upload';
    }
});

function saveToHistory(filename, url, deletionUrl) {
    let history = JSON.parse(localStorage.getItem('uploadHistory') || '[]');
    history.unshift({
        filename,
        url,
        deletionUrl,
        date: new Date().toISOString()
    });

    if (history.length > 50) {
        history = history.slice(0, 50);
    }

    localStorage.setItem('uploadHistory', JSON.stringify(history));
}

function loadHistory() {
    const history = JSON.parse(localStorage.getItem('uploadHistory') || '[]');

    if (history.length === 0) {
        historyBody.innerHTML =
            '<tr><td colspan="4" style="text-align: center; color: #555; padding: 40px;">Нет загруженных файлов</td></tr>';
        return;
    }

    historyBody.innerHTML = history.map((item, index) => {
        const date = new Date(item.date);
        const formattedDate = date.toLocaleString('ru-RU', {
            day: '2-digit',
            month: '2-digit',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });

        return `
            <tr>
                <td class="file-name">${item.filename}</td>
                <td class="file-date">${formattedDate}</td>
                <td><a href="${item.url}" class="file-link" target="_blank">${item.url}</a></td>
                <td>
                    <div class="actions-cell">
                        <button class="copy-btn-table" onclick="copyToClipboard('${item.url}')" title="Копировать">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                            </svg>
                        </button>
                        <button class="delete-btn-table" onclick="deleteFile(${index}, '${item.deletionUrl}')" title="Удалить">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="3 6 5 6 21 6"></polyline>
                                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                            </svg>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');
}

async function deleteFile(index, deletionUrl) {
    if (!confirm('Удалить файл?')) return;

    try {
        const response = await fetch(deletionUrl, { method: 'POST' });

        if (response.ok) {
            let history = JSON.parse(localStorage.getItem('uploadHistory') || '[]');
            history.splice(index, 1);
            localStorage.setItem('uploadHistory', JSON.stringify(history));
            loadHistory();
            showToast('Файл удалён');
        } else {
            showToast('Ошибка удаления');
        }
    } catch (error) {
        showToast('Ошибка: ' + error.message);
    }
}

function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => showToast('Скопировано'));
}

function showToast(message) {
    const toastSpan = toast.querySelector('span');
    toastSpan.textContent = message;
    toast.classList.add('show');

    setTimeout(() => toast.classList.remove('show'), 2000);
}

loadHistory();
