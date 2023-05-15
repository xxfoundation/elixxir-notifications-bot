# notifications-bot

Notifications bot is used to send push notifications to users' devices via firebase.  It continuously loops, polling gateway for users to notify and sending the notifications.  

# Config File

```yaml
# ==================================
# Notification Server Configuration
# ==================================

# START YAML ===
# Verbose logging
logLevel: "${verbose}"
# Path to log file
log: "${log_path}"

# Database connection information
dbUsername: "${db_username}"
dbPassword: "${db_password}"
dbName: "${db_name}"
dbAddress: "${db_address}"

# Path to this server's private key file
keyPath: "${key_path}"
# Path to this server's certificate file
certPath: "${cert_path}"
# The listening port of this server
port: ${port}

# Path to the firebase credentials files
firebaseCredentialsPath: "{fb_creds_path}"
havenFirebaseCredentialsPath: "{fb_creds_path}"

# Path to the permissioning server certificate file
permissioningCertPath: "${permissioning_cert_path}"
# Address:port of the permissioning server
permissioningAddress: "${permissioning_address}:${port}"

# XX Messenger APNS parameters
apnsKeyPath: ""
apnsKeyID: ""
apnsIssuer: ""
apnsBundleID: ""
apnsDev: true

# Haven APNS parameters
havenApnsKeyPath: ""
havenApnsKeyID: ""
havenApnsIssuer: ""
havenApnsBundleID: ""
havenApnsDev: true

# Notification params
notificationRate: 30  # Duration in seconds
notificationsPerBatch: 20
# === END YAML
```
