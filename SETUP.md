# Setup Guide for HomeHub

## Google Cloud Setup

To use Google Calendar and Google Tasks integration with HomeHub, you need to
configure a Google Cloud project, create a service account, and authorize it to
access your data.

## Steps

### 1. Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. If you don't have a project already, create a new one by clicking the
   project dropdown in the top bar and then "New Project".
3. Give your project a name (e.g., "HomeHub Integration") and click "Create".

### 2. Create a Service Account

1. In the Google Cloud Console, with your project selected, navigate to the
   "IAM & Admin" section in the left-hand menu and select "Service Accounts".
2. Click on **+ CREATE SERVICE ACCOUNT** at the top of the page.
3. Enter a name for the service account (e.g., "homehub-service-account") and
   a description.
4. Click **CREATE AND CONTINUE**.
5. You can skip granting roles for now. Click **CONTINUE**.
6. You can also skip granting user access to this service account.
   Click **DONE**.

### 3. Generate a JSON Key

1. After creating the service account, you will be returned to the Service
   Accounts list.
2. Find the service account you just created and click on the three-dot menu
   under the "Actions" column.
3. Select **Manage keys**.
4. Click on **ADD KEY** and then select **Create new key**.
5. Choose **JSON** as the key type and click **CREATE**.
6. A JSON file containing the service account's private key will be downloaded
   to your computer. **Treat this file like a password and keep it secure.**

### 4. Enable Google APIs

For HomeHub to function, you must enable the Google Calendar and Google Tasks
APIs for your project.

1. In the Google Cloud Console, navigate to "APIs & Services" -> "Library".
2. Search for "Google Calendar API" and click on it.
3. Click the **Enable** button.
4. Go back to the API Library.
5. Search for "Google Tasks API" and click on it.
6. Click the **Enable** button.

### 5. Configure HomeHub

1. Copy the downloaded JSON key file to the machine where HomeHub is running.
   A secure, non-public location within the standard configuration directory
   is recommended (e.g., `~/.config/homehub/service-account-key.json`).
2. Open your `config.toml` file (located in `~/.config/homehub/config.toml` or
   the current directory).
3. Find the `[google]` section.
4. Set the `service_account_key_file` option to the full path of the JSON key
   file you just copied:

   ```toml
   [google]
   service_account_key_file = "/home/username/.config/homehub/service-account-key.json"
   ```

### 6. Share Your Resources

The service account now has permission to use the Google APIs, but it doesn't
have access to your personal data yet. You must explicitly grant it access.

The service account has an email address associated with it, which you can find
in the "Service Accounts" section of the IAM & Admin page in your Google Cloud
Console:
`your-service-account-name@your-project-id.iam.gserviceaccount.com`

* **For Google Calendar:**
  1. Go to Google Calendar.
  2. Find the calendar you want HomeHub to display.
  3. Click the three-dot menu next to the calendar name and select "Settings
     and sharing".
  4. Under "Share with specific people or groups", click "Add people".
  5. Enter the service account's email address.
  6. Choose the appropriate permission level (e.g., "See all event details").
  7. Click "Send".
  8. Repeat for any other calendars you want to share.
  9. **Important**: Add the ID of each shared calendar to `config.toml`.
     Find the calendar ID in "Integrate calendar" in calendar settings:

     ```toml
     [google.calendar]
     calendar_ids = ["your-calendar-id@group.calendar.google.com"]
     # Optional: refresh interval in minutes (default: 15)
     calendar_refresh_minutes = 15
     # Optional: time display format (default: "15:04")
     time_format = "15:04"
     ```

* **For Google Tasks:**
  1. Go to Google Tasks (via Gmail, Google Calendar sidebar, or
     https://tasks.google.com/).
  2. Select the task list you want to sync with a HomeHub shopping list.
  3. Click the three-dot menu at the top of the list and select "Share".
  4. Enter the service account email and grant it "Editor" permissions.
  5. Repeat for any other task lists you want to sync.
  6. In your `config.toml`, enable Google Tasks synchronization under
     `[shopping.google_tasks]`. By default, sync is disabled (`enabled = false`):

     ```toml
     [shopping.google_tasks]
     enabled = true
     # How frequently (in minutes) to synchronize with Google Tasks (default: 5)
     refresh_minutes = 5

       # Optional mapping between a store 'name' defined in [[shopping.store]]
       # and the title of a Google Tasks list. If not specified, HomeHub will
       # match a list with the exact same name as the store, or automatically
       # create one if it does not exist.
       [shopping.google_tasks.list_mapping]
       "Walmart" = "My Walmart Shopping List"
       "Costco" = "Urgent Costco Run"
     ```
  7. Google Tasks synchronization runs once immediately at application startup
     and periodically every `refresh_minutes` (defaulting to every 5 minutes).

---

# Home Assistant Integration Setup

HomeHub supports three complementary Home Assistant integration modes (any
combination can be enabled in `config.toml` and Home Assistant).

## 1. MQTT Discovery & Alert Overlays

1. Ensure an MQTT broker (e.g., Mosquitto) is reachable.
2. Enable `[homeassistant.mqtt]` in `config.toml` (when `host` is empty,
   `host`, `port`, `username`, and `password` fall back to
   `[security.frigate_mqtt]`):

   ```toml
   [homeassistant.mqtt]
   enabled = true
   # Optional overrides:
   # host = "mqtt.local"
   # port = 1883
   # username = ""
   # password = ""
   # discovery_prefix = "homeassistant"
   # topic_prefix = "homehub"
   # device_id = "homehub"
   # device_name = "HomeHub"
   ```

3. Restart HomeHub; Home Assistant automatically discovers the **HomeHub**
   device with:
   - `switch` (**Away Mode**), `binary_sensor` (**Display**), `sensor`
     (**Pending Reminders**), `select` (**Active View**), and `button`
     entities (**Restart**, **Next Photo**, **Dismiss Alert**).
4. Publish plain text or JSON to `<topic_prefix>/alert/set` (`homehub/alert/set`
   by default) to pop up a touchscreen alert banner:

   ```json
   {
     "title": "Front Door",
     "message": "Package delivered",
     "level": "info",
     "duration_seconds": 20
   }
   ```

   - `level`: `"info"` (default), `"warning"`, or `"critical"`.
   - `duration_seconds`: defaults to `20`, capped at `300`.
   - Dismiss early via `{"dismiss": true}`, plain text `"dismiss"`, or
     publishing to `homehub/alert/dismiss`.

## 2. HACS Custom Integration (`/api/v1/*`)

1. Optionally set `api_token` under `[app]` in `config.toml` (clients pass
   `Authorization: Bearer <token>` or `X-API-Key: <token>`):

   ```toml
   [app]
   api_token = "your-secret-token"
   ```

2. In Home Assistant, open **HACS** -> **Integrations** -> **Custom
   repositories**, add this repository URL as an **Integration**, and install
   **HomeHub** (`custom_components/homehub`).
3. Restart Home Assistant, then go to **Settings -> Devices & Services -> Add
   Integration -> HomeHub** and enter your HomeHub host, port (`8080`), and
   optional API token.
4. This exposes the **Away Mode** switch, **Display** binary sensor,
   **Pending Reminders** & **Active View** sensors, **Restart** & **Next
   Photo** buttons, one HA **To-do** list per active shopping store, and the
   `homehub.send_alert`, `homehub.dismiss_alert`, and
   `homehub.acknowledge_reminder` services.

## 3. Touchscreen Smart Home Tab

1. Create a Long-Lived Access Token in your Home Assistant user profile.
2. Configure `[homeassistant]` (`url` and `token`) plus at least one
   `[[homeassistant.entity]]` entry in `config.toml` (note: setting `url`
   without `token` fails `--validate-config`):

   ```toml
   [homeassistant]
   url = "http://homeassistant.local:8123"
   token = "your-long-lived-access-token"

   [[homeassistant.entity]]
     entity_id = "light.living_room"
     name = "Living Room"

   [[homeassistant.entity]]
     entity_id = "climate.thermostat"
     name = "Thermostat"
   ```

   Supported interactive domains: `light`, `switch`, `fan`, `input_boolean`,
   `scene`, `script`, `button`, `input_button`, `lock`, `cover`, `climate`.
   Read-only entities such as `sensor` and `binary_sensor` render live state
   tiles.
3. Restart HomeHub to add the Smart Home icon to the left sidebar.
