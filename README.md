# komodo-op

`komodo-op` is a middleware application written in Go that synchronizes secrets from a [1Password vault](https://1password.com/) (using a [1Password Connect server](https://developer.1password.com/docs/connect/)) to a [Komodo](https://komo.do/) instance.

It fetches items from a specified 1Password vault and creates or updates corresponding secret variables in Komodo.

## Functionality

- Connects to a 1Password Connect server and a Komodo instance using environment variables.
- Looks up the 1Password Vault ID based on the provided vault name.
- Lists all items in the specified 1Password vault.
- For each item, iterates through its fields (like username, password, API keys, etc.).
- Creates or updates a secret variable in Komodo for each field using a specific naming convention:
  `OP__KOMODO__<ITEM_NAME>__<FIELD_LABEL>`
- All parts of the name are converted to uppercase.
- Spaces in the item name and field label are replaced with hyphens (`-`).
- The corresponding field value from 1Password is set as the secret value in Komodo.
- Variables created in Komodo are marked as `secret`.

**Example:**
A field labeled `API Key` with value `xyz789` in an item named `My Service API` within the vault named `production` would be synced to Komodo as a secret variable named:
`OP__PRODUCTION__MY-SERVICE-API__API-KEY` with the value `xyz789`.

## What is Komodo?

[Komodo](https://komo.do/) is a web application designed to structure the management of servers, builds, deployments, and automated procedures. It allows you to:

*   Connect and monitor servers (CPU, memory, disk usage) with alerting.
*   Manage Docker containers (create, start, stop, restart, view logs) on connected servers.
*   Deploy Docker Compose stacks defined in the UI or Git repos (with auto-deploy).
*   Build source code into versioned Docker images (auto-build on webhook) using scalable build instances.
*   Manage repositories for automation via scripting/webhooks.
*   Centralize configuration/environment variables with shared secrets and interpolation.
*   Maintain an audit log of all actions.

Komodo has no limits on the number of connected servers or API usage. More information can be found on the [project website](https://komo.do/) and in the [API documentation](https://docs.rs/komodo_client/latest/komodo_client/api/index.html).

## What is 1Password Connect?

[1Password Connect](https://developer.1password.com/docs/connect/) provides a way to access secrets stored in 1Password vaults programmatically, without needing secrets embedded in your applications or configuration files. It runs as a separate service (typically in Docker) that your applications communicate with.

To use `komodo-op`, you need a running 1Password Connect instance linked to your 1Password account. Follow the [1Password Connect Get Started guide](https://developer.1password.com/docs/connect/get-started/) to set it up.

## Configuration

`komodo-op` is configured using environment variables:

- `OP_CONNECT_HOST`: The hostname and port of your 1Password Connect server (e.g., `http://1password-connect:8080` or `https://my-connect.example.com`).
- `OP_VAULT`: The **UUID** of the 1Password vault containing the secrets you want to sync.
- `OP_SERVICE_ACCOUNT_TOKEN`: The API token for your 1Password Connect service account.
- `KOMODO_HOST`: The hostname and port of your Komodo instance (e.g., `http://komodo:8888`).
- `KOMODO_API_KEY`: The API key for authenticating with your Komodo instance.
- `KOMODO_API_SECRET`: The API secret for authenticating with your Komodo instance.
- `LOG_LEVEL`: (Optional) Set the logging verbosity. Options are `DEBUG`, `INFO` (default), `ERROR`. Be careful as `DEBUG` _will_ print your 1password service token in plaintext.

### Runtime Modes and Interval

`komodo-op` can run in two modes:

1.  **One-off Sync (Default):** The application performs a single synchronization run and then exits. This is the default behavior.
2.  **Daemon Mode (`-daemon`):** The application runs continuously, performing an initial sync immediately and then repeating the sync periodically.

The synchronization interval in daemon mode is controlled by:

- **`-interval` flag:** Command-line flag specifying the duration between syncs (e.g., `-interval=5m`, `-interval=2h30s`). This takes precedence.
- **`SYNC_INTERVAL` environment variable:** Sets the interval if the `-interval` flag is not provided. Accepts duration strings (e.g., `1h`, `30m`, `90s`). Defaults to `1h` in the Docker image.

### Running with Docker Compose (Recommended)

A `docker-compose.yaml` file is provided to simplify running `komodo-op` alongside the required 1Password Connect services.

**Prerequisites:**

1.  **Install Docker and Docker Compose.**
2.  **Set up 1Password Connect:** Follow the [1Password Connect Get Started guide](https://developer.1password.com/docs/connect/get-started/). You will need to:
    *   Create a 1Password Connect Server in your 1Password account.
    *   Download the `1password-credentials.json` file and place it in the same directory as the `docker-compose.yaml` file.
    *   Create a Service Account token with access to the vault you want to sync.

**Steps:**

1.  **Configure Environment Variables:** Edit the `environment` section for the `komodo-op` service within the `docker-compose.yaml` file:
    *   Set `KOMODO_HOST`, `KOMODO_API_KEY`, and `KOMODO_API_SECRET` for your Komodo instance.
    *   Set `OP_SERVICE_ACCOUNT_TOKEN` to the token you generated.
    *   Set `OP_VAULT` to the **UUID** of the 1Password vault you wish to sync.
    *   (Optional) Adjust `SYNC_INTERVAL` or `LOG_LEVEL`.

2.  **Run Docker Compose:**
    ```bash
    # Build the komodo-op image and start all services
    docker compose up --build -d
    ```

3.  **View Logs:**
    ```bash
    docker compose logs -f komodo-op
    docker compose logs -f op-connect-api
    ```

4.  **Stop Services:**
    ```bash
    docker compose down
    ```

### Running with Docker (Standalone)

1.  **Build the Docker image:**
    ```