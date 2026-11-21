"""Async HTTP API client for HomeHub."""

from __future__ import annotations

import asyncio
from typing import Any

import aiohttp


class HomeHubApiError(Exception):
    """Base exception for HomeHub API errors."""


class HomeHubAuthError(HomeHubApiError):
    """Raised when API authentication fails."""


class HomeHubConnectionError(HomeHubApiError):
    """Raised when connecting to HomeHub fails."""


class HomeHubApiClient:
    """Client for interacting with the HomeHub /api/v1 HTTP API."""

    def __init__(
        self,
        session: aiohttp.ClientSession,
        host: str,
        port: int = 8080,
        api_token: str | None = None,
    ) -> None:
        self._session = session
        self._base_url = f"http://{host}:{port}"
        self._api_token = api_token or ""

    def _headers(self) -> dict[str, str]:
        headers = {"Accept": "application/json"}
        if self._api_token:
            headers["Authorization"] = f"Bearer {self._api_token}"
        return headers

    async def _request(
        self,
        method: str,
        path: str,
        json_data: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        url = f"{self._base_url}{path}"
        try:
            async with asyncio.timeout(10):
                resp = await self._session.request(
                    method,
                    url,
                    json=json_data,
                    headers=self._headers(),
                )
        except (aiohttp.ClientError, TimeoutError) as err:
            raise HomeHubConnectionError(
                f"Error communicating with HomeHub at {url}: {err}"
            ) from err

        if resp.status == 401:
            raise HomeHubAuthError("Invalid or missing HomeHub API token")
        if resp.status >= 400:
            body = await resp.text()
            raise HomeHubApiError(
                f"HomeHub API error {resp.status} for {path}: {body}"
            )
        return await resp.json()

    async def async_get_status(self) -> dict[str, Any]:
        """Fetch overall HomeHub dashboard status."""
        return await self._request("GET", "/api/v1/status")

    async def async_set_away(self, away: bool) -> dict[str, Any]:
        """Enable or disable persistent Away mode."""
        return await self._request("POST", "/api/v1/away", {"away": away})

    async def async_restart(self) -> dict[str, Any]:
        """Trigger an in-place application restart."""
        return await self._request("POST", "/api/v1/restart")

    async def async_next_photo(self) -> dict[str, Any]:
        """Advance the home slideshow to the next photo."""
        return await self._request("POST", "/api/v1/next-photo")

    async def async_send_alert(
        self,
        message: str,
        title: str = "Home Assistant",
        level: str = "info",
        duration_seconds: int = 20,
    ) -> dict[str, Any]:
        """Display an alert overlay on the HomeHub touchscreen."""
        return await self._request(
            "POST",
            "/api/v1/alert",
            {
                "title": title,
                "message": message,
                "level": level,
                "duration_seconds": duration_seconds,
            },
        )

    async def async_dismiss_alert(self) -> dict[str, Any]:
        """Dismiss any active Home Assistant alert overlay."""
        return await self._request("POST", "/api/v1/alert", {"dismiss": True})

    async def async_get_shopping(self) -> dict[str, Any]:
        """Fetch shopping stores and their items."""
        return await self._request("GET", "/api/v1/shopping")

    async def async_add_shopping_item(
        self,
        store_id: int,
        name: str,
        quantity: int = 1,
        checked: bool = False,
    ) -> dict[str, Any]:
        """Add an item to a HomeHub shopping list."""
        return await self._request(
            "POST",
            "/api/v1/shopping/items",
            {
                "store_id": store_id,
                "name": name,
                "quantity": quantity,
                "checked": checked,
            },
        )

    async def async_update_shopping_item(
        self,
        item_id: int,
        *,
        name: str | None = None,
        quantity: int | None = None,
        checked: bool | None = None,
    ) -> dict[str, Any]:
        """Update an existing HomeHub shopping list item."""
        payload: dict[str, Any] = {}
        if name is not None:
            payload["name"] = name
        if quantity is not None:
            payload["quantity"] = quantity
        if checked is not None:
            payload["checked"] = checked
        return await self._request(
            "PATCH", f"/api/v1/shopping/items/{item_id}", payload
        )

    async def async_delete_shopping_item(self, item_id: int) -> dict[str, Any]:
        """Delete a shopping list item."""
        return await self._request("DELETE", f"/api/v1/shopping/items/{item_id}")

    async def async_get_reminders(self) -> dict[str, Any]:
        """Fetch household reminders and pending count."""
        return await self._request("GET", "/api/v1/reminders")

    async def async_acknowledge_reminder(self, reminder_id: int) -> dict[str, Any]:
        """Acknowledge a triggered reminder."""
        return await self._request(
            "POST", f"/api/v1/reminders/{reminder_id}/acknowledge"
        )
