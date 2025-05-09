// Fetch list of files from server
function refreshFileList() {
    console.log('refreshFileList function called');
    
    // Check if addLogMessage function exists
    if (typeof addLogMessage !== 'function') {
        console.error('addLogMessage function is not defined');
        // Create a fallback function to prevent errors
        window.addLogMessage = function(message, type) {
            console.log(`[${type}] ${message}`);
        };
    }
    
    const storageType = document.getElementById('listStorageType').value;
    let prefix = '';
    let apiUrl = '/api/list?storageType=' + storageType;
    
    // Get storage provider specific parameters
    if (storageType === 's3') {
        const region = document.getElementById('s3ListRegion').value;
        const bucket = document.getElementById('s3ListBucket').value;
        const accessKey = document.getElementById('s3ListAccessKey').value;
        const secretKey = document.getElementById('s3ListSecretKey').value;
        prefix = document.getElementById('s3ListPrefix').value || '';
        
        apiUrl += `&region=${encodeURIComponent(region)}&bucket=${encodeURIComponent(bucket)}&accessKey=${encodeURIComponent(accessKey)}&secretKey=${encodeURIComponent(secretKey)}`;
    } else if (storageType === 'google') {
        const bucket = document.getElementById('gcsListBucket').value;
        const credFile = document.getElementById('gcsListCredFile').value;
        prefix = document.getElementById('gcsListPrefix').value || '';
        
        apiUrl += `&bucket=${encodeURIComponent(bucket)}&credentialFile=${encodeURIComponent(credFile)}`;
    }
    
    if (prefix) {
        apiUrl += `&prefix=${encodeURIComponent(prefix)}`;
    }
    
    console.log('File list URL:', apiUrl);
    addLogMessage('Fetching file list...', 'info');
    
    // Show loading indication
    document.getElementById('fileList').innerHTML = '<tr><td colspan="4" class="text-center">Loading files...</td></tr>';
    
    // Fetch files from API
    fetch(apiUrl)
        .then(response => response.json())
        .then(data => {
            console.log('File list response:', data);
            
            if (data.success) {
                if (data.data && data.data.length > 0) {
                    const fileList = document.getElementById('fileList');
                    fileList.innerHTML = '';
                    
                    // Sort files by modified date (most recent first)
                    data.data.sort((a, b) => {
                        const dateA = a.UploadedAt ? new Date(a.UploadedAt) : new Date(0);
                        const dateB = b.UploadedAt ? new Date(b.UploadedAt) : new Date(0);
                        return dateB - dateA;
                    });
                    
                    // Add files to table
                    data.data.forEach(file => {
                        const row = document.createElement('tr');
                        
                        // Format file size
                        let fileSize = file.Size;
                        let sizeUnit = 'B';
                        if (fileSize >= 1024 * 1024 * 1024) {
                            fileSize = (fileSize / (1024 * 1024 * 1024)).toFixed(2);
                            sizeUnit = 'GB';
                        } else if (fileSize >= 1024 * 1024) {
                            fileSize = (fileSize / (1024 * 1024)).toFixed(2);
                            sizeUnit = 'MB';
                        } else if (fileSize >= 1024) {
                            fileSize = (fileSize / 1024).toFixed(2);
                            sizeUnit = 'KB';
                        }
                        
                        const formattedDate = file.UploadedAt ? new Date(file.UploadedAt).toLocaleString() : 'Unknown';
                        
                        row.innerHTML = `
                            <td>${file.Name}</td>
                            <td>${file.ContentType || 'Unknown'}</td>
                            <td>${fileSize} ${sizeUnit}</td>
                            <td>
                                <div class="btn-group btn-group-sm">
                                    <a href="/api/download?id=${file.StorageID}&storageType=${file.StorageType}" 
                                       class="btn btn-primary" target="_blank">Download</a>
                                    <button class="btn btn-danger delete-file" 
                                            data-id="${file.StorageID}" 
                                            data-storage-type="${file.StorageType}" 
                                            data-file-name="${file.Name}">Delete</button>
                                </div>
                            </td>
                        `;
                        
                        fileList.appendChild(row);
                        
                        // Add event listener for delete button
                        row.querySelector('.delete-file').addEventListener('click', function() {
                            const fileId = this.getAttribute('data-id');
                            const storageType = this.getAttribute('data-storage-type');
                            const fileName = this.getAttribute('data-file-name');
                            
                            if (confirm(`Are you sure you want to delete "${fileName}"?`)) {
                                deleteFile(fileId, storageType, fileName);
                            }
                        });
                    });
                    
                    addLogMessage(`Found ${data.data.length} files`, 'info');
                } else {
                    document.getElementById('fileList').innerHTML = 
                        '<tr><td colspan="4" class="text-center">No files found</td></tr>';
                    addLogMessage('No files found', 'info');
                }
            } else {
                document.getElementById('fileList').innerHTML = 
                    `<tr><td colspan="4" class="text-center">Error: ${data.error || 'Unknown error'}</td></tr>`;
                addLogMessage(`Failed to load files: ${data.error || 'Unknown error'}`, 'error');
            }
        })
        .catch(error => {
            console.error('Error fetching file list:', error);
            document.getElementById('fileList').innerHTML = 
                `<tr><td colspan="4" class="text-center">Error loading files: ${error.message}</td></tr>`;
            addLogMessage(`Error loading files: ${error.message}`, 'error');
        });
}

// Delete a file
function deleteFile(fileId, storageType, fileName) {
    const apiUrl = `/api/delete?id=${encodeURIComponent(fileId)}&storageType=${encodeURIComponent(storageType)}`;
    
    fetch(apiUrl, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            addLogMessage(`Deleted file: ${fileName}`, 'success');
            refreshFileList(); // Refresh the file list
        } else {
            addLogMessage(`Failed to delete file: ${data.error || 'Unknown error'}`, 'error');
        }
    })
    .catch(error => {
        console.error('Error deleting file:', error);
        addLogMessage(`Error deleting file: ${error.message}`, 'error');
    });
}
