# Google Cloud Setup for HomeHub

To use Google Calendar and Google Drive integration with HomeHub, you need to configure a Google Cloud project, create a service account, and authorize it to access your data.

## Steps

### 1. Create a Google Cloud Project

1.  Go to the [Google Cloud Console](https://console.cloud.google.com/).
2.  If you don't have a project already, create a new one by clicking the project dropdown in the top bar and then "New Project".
3.  Give your project a name (e.g., "HomeHub Integration") and click "Create".

### 2. Create a Service Account

1.  In the Google Cloud Console, with your project selected, navigate to the "IAM & Admin" section in the left-hand menu and select "Service Accounts".
2.  Click on **+ CREATE SERVICE ACCOUNT** at the top of the page.
3.  Enter a name for the service account (e.g., "homehub-service-account") and a description.
4.  Click **CREATE AND CONTINUE**.
5.  You can skip granting roles for now. Click **CONTINUE**.
6.  You can also skip granting user access to this service account. Click **DONE**.

### 3. Generate a JSON Key

1.  After creating the service account, you will be returned to the Service Accounts list.
2.  Find the service account you just created and click on the three-dot menu under the "Actions" column.
3.  Select **Manage keys**.
4.  Click on **ADD KEY** and then select **Create new key**.
5.  Choose **JSON** as the key type and click **CREATE**.
6.  A JSON file containing the service account's private key will be downloaded to your computer. **Treat this file like a password and keep it secure.**

### 4. Enable Google APIs

For HomeHub to function, you must enable the Google Drive and Google Calendar APIs for your project.

1.  In the Google Cloud Console, navigate to "APIs & Services" -> "Library".
2.  Search for "Google Drive API" and click on it.
3.  Click the **Enable** button. If it's already enabled, you don't need to do anything.
4.  Go back to the API Library.
5.  Search for "Google Calendar API" and click on it.
6.  Click the **Enable** button.
7.  Go back to the API Library.
8.  Search for "Google Tasks API" and click on it.
9.  Click the **Enable** button.

### 5. Configure HomeHub

1.  Copy the downloaded JSON key file to the machine where HomeHub is running. A secure, non-public location is recommended (e.g., inside the configuration directory).
2.  Open your `config.toml` file.
3.  Find the `[google]` section.
4.  Set the `service_account_key_file` option to the full path of the JSON key file you just copied.

    ```toml
    [google]
    service_account_key_file = "/path/to/your/service-account-key.json"
    ```

### 6. Share Your Resources

The service account now has permission to use the Google APIs, but it doesn't have access to your personal data yet. You must explicitly grant it access.

The service account has an email address associated with it, which you can find in the "Service Accounts" section of the IAM & Admin page in your Google Cloud Console. It will look something like `your-service-account-name@your-project-id.iam.gserviceaccount.com`.

*   **For Google Drive:**
    1.  Go to Google Drive.
    2.  Find the folder you want HomeHub to sync photos from.
    3.  Right-click the folder and select "Share".
    4.  Enter the service account's email address in the "Add people and groups" field.
    5.  Give it at least "Viewer" permissions. If you want HomeHub to delete photos after downloading, you will need to grant it "Editor" permissions.
    6.  Click "Send" (you can uncheck "Notify people").

*   **For Google Calendar:**
    1.  Go to Google Calendar.
    2.  Find the calendar you want HomeHub to display.
    3.  Click the three-dot menu next to the calendar name and select "Settings and sharing".
    4.  Under "Share with specific people or groups", click "Add people".
    5.  Enter the service account's email address.
    6.  Choose the appropriate permission level (e.g., "See all event details").
    7.  Click "Send".
    8.  Repeat for any other calendars you want to share.
    9.  **Important**: You must add the ID of each shared calendar to your `config.toml` file. You can find the calendar ID in the "Integrate calendar" section of the calendar's settings.

        ```toml
        [google.calendar]
        calendar_ids = ["your-calendar-id@group.calendar.google.com"]
        ```

*   **For Google Tasks:**
    1.  Go to Google Tasks (you can access it via the sidebar in Gmail or Google Calendar, or at [https://tasks.google.com/](https://tasks.google.com/)).
    2.  Select the task list you want to sync with a HomeHub shopping list.
    3.  Click the three-dot menu at the top of the list and select "Share".
    4.  Enter the service account's email address and grant it "Editor" permissions.
    5.  Repeat for any other task lists you want to sync.
    6.  In your `config.toml`, you can optionally map your store names to specific task list names under `[shopping.google_tasks.list_mapping]`. If you don't provide a mapping, HomeHub will try to find a task list with the same name as your store.
