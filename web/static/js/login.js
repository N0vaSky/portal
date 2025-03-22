/**
 * Fibratus Portal - Login JavaScript
 */

$(document).ready(function() {
    // Handle login form submission
    $('#login').submit(function(e) {
        e.preventDefault();
        
        // Hide any previous error messages
        $('#error-message').addClass('hidden');
        
        // Get form data
        const username = $('#username').val();
        const password = $('#password').val();
        const otp = $('#otp').val();
        
        // Validate form
        if (!username || !password) {
            showError('Username and password are required');
            return;
        }
        
        // Send login request
        $.ajax({
            url: '/api/v1/login',
            type: 'POST',
            contentType: 'application/json',
            data: JSON.stringify({
                username: username,
                password: password,
                otp: otp
            }),
            success: function(response) {
                // Check if MFA is required
                if (response.mfa_required) {
                    if (response.mfa_setup) {
                        // Show MFA setup UI
                        showMFASetup(response.mfa_setup);
                    } else {
                        // Show OTP field
                        $('#otp-field').removeClass('hidden');
                    }
                } else {
                    // Login successful, store token and redirect
                    localStorage.setItem('auth_token', response.token);
                    localStorage.setItem('user', JSON.stringify(response.user));
                    window.location.href = '/dashboard';
                }
            },
            error: function(xhr) {
                // Show error message
                if (xhr.responseText) {
                    showError(xhr.responseText);
                } else {
                    showError('Login failed. Please check your credentials.');
                }
            }
        });
    });
    
    // Handle MFA verification
    $('#verify-mfa').click(function() {
        const otp = $('#verify-otp').val();
        
        if (!otp) {
            showError('Please enter the verification code');
            return;
        }
        
        // Send verification request
        $.ajax({
            url: '/api/v1/mfa/verify',
            type: 'POST',
            headers: {
                'Authorization': 'Bearer ' + localStorage.getItem('temp_token')
            },
            contentType: 'application/json',
            data: JSON.stringify({
                otp: otp
            }),
            success: function(response) {
                // MFA setup successful, store token and redirect
                localStorage.removeItem('temp_token');
                localStorage.setItem('auth_token', response.token);
                localStorage.setItem('user', JSON.stringify(response.user));
                window.location.href = '/dashboard';
            },
            error: function(xhr) {
                // Show error message
                if (xhr.responseText) {
                    showError(xhr.responseText);
                } else {
                    showError('Verification failed. Please check your code.');
                }
            }
        });
    });
    
    // Show error message
    function showError(message) {
        $('#error-message').removeClass('hidden').text(message);
    }
    
    // Show MFA setup UI
    function showMFASetup(setupData) {
        // Store temporary token
        localStorage.setItem('temp_token', setupData.token);
        
        // Show MFA setup section
        $('#login-form').addClass('hidden');
        $('#mfa-setup').removeClass('hidden');
        
        // Set MFA secret
        $('#mfa-secret').text(setupData.secret);
        
        // Generate QR code
        new QRCode(document.getElementById('qrcode'), {
            text: setupData.url,
            width: 200,
            height: 200
        });
    }
    
    // Check if we're already logged in
    const token = localStorage.getItem('auth_token');
    if (token) {
        // Verify the token is valid
        $.ajax({
            url: '/api/v1/users/me',
            type: 'GET',
            headers: {
                'Authorization': 'Bearer ' + token
            },
            success: function() {
                // Token is valid, redirect to dashboard
                window.location.href = '/dashboard';
            },
            error: function() {
                // Token is invalid, clear storage
                localStorage.removeItem('auth_token');
                localStorage.removeItem('user');
            }
        });
    }
});