"""Unit tests for the HomeHub HACS custom integration."""

from __future__ import annotations

import asyncio
import importlib
import json
from pathlib import Path
import sys
import types
import unittest

REPO_ROOT = Path(__file__).resolve().parents[3]
COMP_DIR = REPO_ROOT / "custom_components" / "homehub"


def _install_ha_stubs() -> None:
    """Install lightweight HA & aiohttp stubs when running outside HA."""
    if "aiohttp" not in sys.modules:
        aiohttp = types.ModuleType("aiohttp")

        class ClientError(Exception):
            pass

        class ClientSession:
            pass

        aiohttp.ClientError = ClientError
        aiohttp.ClientSession = ClientSession
        sys.modules["aiohttp"] = aiohttp

    if "voluptuous" not in sys.modules:
        vol = types.ModuleType("voluptuous")
        vol.Schema = lambda x: x
        vol.Required = lambda k, default=None: k
        vol.Optional = lambda k, default=None: k
        vol.All = lambda *args: args
        vol.Range = lambda **kwargs: kwargs
        sys.modules["voluptuous"] = vol

    if "homeassistant" in sys.modules:
        return

    def _make_pkg(name: str) -> types.ModuleType:
        mod = types.ModuleType(name)
        mod.__path__ = []
        sys.modules[name] = mod
        return mod

    _make_pkg("homeassistant")
    const = _make_pkg("homeassistant.const")
    const.CONF_HOST = "host"
    const.CONF_PORT = "port"

    core = _make_pkg("homeassistant.core")
    core.HomeAssistant = object
    core.ServiceCall = object
    core.callback = lambda fn: fn

    exc = _make_pkg("homeassistant.exceptions")
    exc.ConfigEntryAuthFailed = type("ConfigEntryAuthFailed", (Exception,), {})

    ce = _make_pkg("homeassistant.config_entries")
    ce.ConfigEntry = object
    ce.ConfigFlowResult = dict

    class ConfigFlow:
        def __init_subclass__(cls, **kwargs):
            pass

    ce.ConfigFlow = ConfigFlow
    ce.OptionsFlow = object

    helpers = _make_pkg("homeassistant.helpers")
    _ = helpers
    aio_helper = _make_pkg("homeassistant.helpers.aiohttp_client")
    aio_helper.async_get_clientsession = lambda hass: None

    dev_reg = _make_pkg("homeassistant.helpers.device_registry")
    dev_reg.DeviceInfo = dict

    ent_plat = _make_pkg("homeassistant.helpers.entity_platform")
    ent_plat.AddEntitiesCallback = object

    uc = _make_pkg("homeassistant.helpers.update_coordinator")
    uc.UpdateFailed = type("UpdateFailed", (Exception,), {})

    class DataUpdateCoordinator:
        def __class_getitem__(cls, item):
            return cls

        def __init__(self, hass, logger, name=None, update_interval=None):
            self.hass = hass
            self.logger = logger
            self.name = name
            self.update_interval = update_interval
            self.data = None

    class CoordinatorEntity:
        def __class_getitem__(cls, item):
            return cls

        def __init__(self, coordinator):
            self.coordinator = coordinator

    uc.DataUpdateCoordinator = DataUpdateCoordinator
    uc.CoordinatorEntity = CoordinatorEntity

    _make_pkg("homeassistant.components")

    sw = _make_pkg("homeassistant.components.switch")
    sw.SwitchEntity = type("SwitchEntity", (), {})

    bs = _make_pkg("homeassistant.components.binary_sensor")
    bs.BinarySensorEntity = type("BinarySensorEntity", (), {})
    bs.BinarySensorDeviceClass = types.SimpleNamespace(POWER="power")

    se = _make_pkg("homeassistant.components.sensor")
    se.SensorEntity = type("SensorEntity", (), {})

    btn = _make_pkg("homeassistant.components.button")
    btn.ButtonEntity = type("ButtonEntity", (), {})
    btn.ButtonDeviceClass = types.SimpleNamespace(RESTART="restart")

    todo = _make_pkg("homeassistant.components.todo")

    class TodoItemStatus:
        NEEDS_ACTION = "needs_action"
        COMPLETED = "completed"

    class TodoItem:
        def __init__(self, uid=None, summary=None, status=None, description=None):
            self.uid = uid
            self.summary = summary
            self.status = status
            self.description = description

    class TodoListEntityFeature:
        CREATE_TODO_ITEM = 1
        DELETE_TODO_ITEM = 2
        UPDATE_TODO_ITEM = 4
        SET_DESCRIPTION_ON_ITEM = 8

    todo.TodoItem = TodoItem
    todo.TodoItemStatus = TodoItemStatus
    todo.TodoListEntity = type("TodoListEntity", (), {})
    todo.TodoListEntityFeature = TodoListEntityFeature


_install_ha_stubs()
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))


class _FakeResponse:
    def __init__(self, status: int, payload):
        self.status = status
        self._payload = payload

    async def json(self):
        return self._payload

    async def text(self):
        return json.dumps(self._payload)


class _FakeSession:
    def __init__(self, handler):
        self._handler = handler
        self.calls = []

    async def request(self, method, url, json=None, headers=None):
        self.calls.append((method, url, json, headers))
        return self._handler(method, url, json, headers)


class HomeHubIntegrationTests(unittest.TestCase):
    def test_json_manifests_valid(self):
        hacs = json.loads((REPO_ROOT / "hacs.json").read_text())
        self.assertEqual(hacs["name"], "HomeHub")

        manifest = json.loads((COMP_DIR / "manifest.json").read_text())
        self.assertEqual(manifest["domain"], "homehub")
        self.assertTrue(manifest["config_flow"])

        strings = json.loads((COMP_DIR / "strings.json").read_text())
        en = json.loads((COMP_DIR / "translations" / "en.json").read_text())
        self.assertEqual(strings, en)

    def test_api_client_requests_and_errors(self):
        api = importlib.import_module("custom_components.homehub.api")

        def handler(method, url, payload, headers):
            self.assertEqual(headers.get("Authorization"), "Bearer secret")
            if url.endswith("/api/v1/status"):
                return _FakeResponse(200, {"away": False, "display_on": True})
            if url.endswith("/api/v1/away"):
                return _FakeResponse(200, {"away": payload["away"]})
            if url.endswith("/api/v1/shopping/items"):
                return _FakeResponse(201, {"id": 9, "name": payload["name"]})
            return _FakeResponse(401, {"error": "unauthorized"})

        session = _FakeSession(handler)
        client = api.HomeHubApiClient(session, "127.0.0.1", 8080, "secret")

        async def run():
            st = await client.async_get_status()
            self.assertTrue(st["display_on"])
            away = await client.async_set_away(True)
            self.assertTrue(away["away"])
            item = await client.async_add_shopping_item(1, "Milk", 2)
            self.assertEqual(item["id"], 9)
            with self.assertRaises(api.HomeHubAuthError):
                await client.async_get_reminders()

        asyncio.run(run())

    def test_entities_and_todo_mapping(self):
        switch_mod = importlib.import_module("custom_components.homehub.switch")
        bs_mod = importlib.import_module(
            "custom_components.homehub.binary_sensor"
        )
        sensor_mod = importlib.import_module("custom_components.homehub.sensor")
        todo_mod = importlib.import_module("custom_components.homehub.todo")

        fake_coord = types.SimpleNamespace(
            data={
                "status": {
                    "away": True,
                    "display_on": False,
                    "active_view": "home",
                },
                "stores": [
                    {
                        "id": 1,
                        "name": "Costco",
                        "items": [
                            {
                                "id": 42,
                                "name": "Coffee",
                                "quantity": 2,
                                "checked": False,
                            },
                            {
                                "id": 43,
                                "name": "Eggs",
                                "quantity": 1,
                                "checked": True,
                            },
                        ],
                    }
                ],
                "reminders": {
                    "pending_count": 1,
                    "reminders": [
                        {
                            "id": 7,
                            "title": "Take vitamins",
                            "time": "08:00",
                            "enabled": True,
                            "acknowledged": False,
                            "last_triggered": "2026-11-20T08:00:00Z",
                        }
                    ],
                },
            }
        )

        sw = switch_mod.HomeHubAwaySwitch(fake_coord, "entry1")
        self.assertTrue(sw.is_on)

        disp = bs_mod.HomeHubDisplayBinarySensor(fake_coord, "entry1")
        self.assertFalse(disp.is_on)

        rem_sensor = sensor_mod.HomeHubPendingRemindersSensor(
            fake_coord, "entry1"
        )
        self.assertEqual(rem_sensor.native_value, 1)
        self.assertEqual(
            rem_sensor.extra_state_attributes["pending"][0]["title"],
            "Take vitamins",
        )

        view_sensor = sensor_mod.HomeHubActiveViewSensor(fake_coord, "entry1")
        self.assertEqual(view_sensor.native_value, "home")

        todo_entity = todo_mod.HomeHubShoppingTodoListEntity(
            fake_coord, "entry1", 1, "Costco"
        )
        items = todo_entity.todo_items
        self.assertEqual(len(items), 2)
        self.assertEqual(items[0].summary, "Coffee")
        self.assertEqual(items[0].description, "Qty: 2")
        self.assertEqual(items[0].status, "needs_action")
        self.assertEqual(items[1].status, "completed")
        self.assertEqual(todo_mod.parse_quantity_from_description("Qty: 4"), 4)
        self.assertEqual(todo_mod.parse_quantity_from_description(None), 1)


if __name__ == "__main__":
    unittest.main()
