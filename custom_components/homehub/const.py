"""Constants for the HomeHub integration."""

DOMAIN = "homehub"

CONF_API_TOKEN = "api_token"
CONF_SCAN_INTERVAL = "scan_interval"

DEFAULT_PORT = 8080
DEFAULT_SCAN_INTERVAL = 30

SERVICE_SEND_ALERT = "send_alert"
SERVICE_DISMISS_ALERT = "dismiss_alert"
SERVICE_ACKNOWLEDGE_REMINDER = "acknowledge_reminder"

PLATFORMS = [
    "switch",
    "binary_sensor",
    "sensor",
    "button",
    "todo",
]
