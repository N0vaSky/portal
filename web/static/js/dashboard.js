/**
 * Fibratus Portal - Dashboard JavaScript
 */

$(document).ready(function() {
    // Check authentication
    const token = localStorage.getItem('auth_token');
    if (!token) {
        // Not logged in, redirect to login page
        window.location.href = '/login';
        return;
    }
    
    // Set up AJAX defaults
    $.ajaxSetup({
        headers: {
            'Authorization': 'Bearer ' + token
        }
    });
    
    // Load user info
    const user = JSON.parse(localStorage.getItem('user'));
    if (user) {
        $('#current-user').text(user.username);
    }
    
    // Handle logout
    $('#logout-button').click(function() {
        localStorage.removeItem('auth_token');
        localStorage.removeItem('user');
        window.location.href = '/login';
    });
    
    // Load dashboard data
    loadDashboardData();
    
    // Auto-refresh data every 60 seconds
    setInterval(loadDashboardData, 60000);
});

/**
 * Load dashboard data from API
 */
function loadDashboardData() {
    // Load node statistics
    $.ajax({
        url: '/api/v1/nodes/stats',
        type: 'GET',
        success: function(response) {
            $('#total-nodes').text(response.total || 0);
            $('#online-nodes').text(response.online || 0);
            $('#isolated-nodes').text(response.isolated || 0);
            
            // Update node status table
            updateNodeStatusTable(response.recent_nodes || []);
        },
        error: handleAjaxError
    });
    
    // Load alert statistics
    $.ajax({
        url: '/api/v1/alerts/stats',
        type: 'GET',
        success: function(response) {
            $('#recent-alerts').text(response.total_recent || 0);
            
            // Update recent alerts table
            updateRecentAlertsTable(response.recent_alerts || []);
        },
        error: handleAjaxError
    });
    
    // Load command history
    $.ajax({
        url: '/api/v1/commands?limit=5',
        type: 'GET',
        success: function(response) {
            // Update recent commands table
            updateRecentCommandsTable(response.commands || []);
        },
        error: handleAjaxError
    });
}

/**
 * Update the node status table with data
 */
function updateNodeStatusTable(nodes) {
    const table = $('#node-status-table');
    table.empty();
    
    if (nodes.length === 0) {
        table.append('<tr><td colspan="4" class="px-6 py-4 text-center text-gray-500">No nodes found</td></tr>');
        return;
    }
    
    nodes.forEach(function(node) {
        const statusClass = node.status === 'online' ? 'text-green-500' : 'text-red-500';
        const isolatedBadge = node.isolated ? 
            '<span class="ml-2 px-2 py-1 text-xs bg-yellow-100 text-yellow-800 rounded-full">Isolated</span>' : '';
        
        const row = `
            <tr>
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm font-medium text-gray-900">${node.hostname}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm text-gray-500">${node.ip_address}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm ${statusClass}">${node.status}${isolatedBadge}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    ${formatTimestamp(node.last_heartbeat)}
                </td>
            </tr>
        `;
        
        table.append(row);
    });
}

/**
 * Update the recent alerts table with data
 */
function updateRecentAlertsTable(alerts) {
    const table = $('#recent-alerts-table');
    table.empty();
    
    if (alerts.length === 0) {
        table.append('<tr><td colspan="6" class="px-6 py-4 text-center text-gray-500">No recent alerts</td></tr>');
        return;
    }
    
    alerts.forEach(function(alert) {
        let severityClass = 'text-gray-800';
        let severityBg = 'bg-gray-100';
        
        switch (alert.severity.toLowerCase()) {
            case 'critical':
                severityClass = 'text-red-800';
                severityBg = 'bg-red-100';
                break;
            case 'high':
                severityClass = 'text-orange-800';
                severityBg = 'bg-orange-100';
                break;
            case 'medium':
                severityClass = 'text-yellow-800';
                severityBg = 'bg-yellow-100';
                break;
            case 'low':
                severityClass = 'text-blue-800';
                severityBg = 'bg-blue-100';
                break;
            case 'info':
                severityClass = 'text-green-800';
                severityBg = 'bg-green-100';
                break;
        }
        
        const statusBadge = alert.acknowledged ? 
            '<span class="px-2 py-1 text-xs bg-green-100 text-green-800 rounded-full">Acknowledged</span>' :
            '<span class="px-2 py-1 text-xs bg-red-100 text-red-800 rounded-full">New</span>';
        
        const row = `
            <tr>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    ${formatTimestamp(alert.timestamp)}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <span class="px-2 py-1 text-xs ${severityBg} ${severityClass} rounded-full">
                        ${alert.severity}
                    </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    ${alert.hostname || '-'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    ${alert.rule_name || '-'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm font-medium text-gray-900">${alert.title}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm">
                    ${statusBadge}
                </td>
            </tr>
        `;
        
        table.append(row);
    });
}

/**
 * Update the recent commands table with data
 */
function updateRecentCommandsTable(commands) {
    const table = $('#recent-commands-table');
    table.empty();
    
    if (commands.length === 0) {
        table.append('<tr><td colspan="4" class="px-6 py-4 text-center text-gray-500">No recent commands</td></tr>');
        return;
    }
    
    commands.forEach(function(command) {
        let statusClass = 'text-gray-800';
        let statusBg = 'bg-gray-100';
        
        switch (command.status.toLowerCase()) {
            case 'success':
                statusClass = 'text-green-800';
                statusBg = 'bg-green-100';
                break;
            case 'failed':
                statusClass = 'text-red-800';
                statusBg = 'bg-red-100';
                break;
            case 'pending':
                statusClass = 'text-yellow-800';
                statusBg = 'bg-yellow-100';
                break;
        }
        
        const row = `
            <tr>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    ${formatTimestamp(command.executed_at)}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    ${command.hostname || '-'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm font-medium text-gray-900">${command.command_type}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <span class="px-2 py-1 text-xs ${statusBg} ${statusClass} rounded-full">
                        ${command.status}
                    </span>
                </td>
            </tr>
        `;
        
        table.append(row);
    });
}

/**
 * Format a timestamp in a human-readable format
 */
function formatTimestamp(timestamp) {
    if (!timestamp) return '-';
    
    const date = new Date(timestamp);
    return date.toLocaleString();
}

/**
 * Handle AJAX errors
 */
function handleAjaxError(xhr) {
    console.error('API Error:', xhr.responseText);
    
    // Check if the error is due to authentication
    if (xhr.status === 401) {
        // Authentication error, redirect to login
        localStorage.removeItem('auth_token');
        localStorage.removeItem('user');
        window.location.href = '/login';
    }
}