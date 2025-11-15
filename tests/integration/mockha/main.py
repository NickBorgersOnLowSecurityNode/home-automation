#!/usr/bin/env python3
"""
Mock Home Assistant Service for Integration Testing

This service simulates the Home Assistant WebSocket API for testing purposes.
It maintains state for entities, accepts service calls, and emits events.
"""

import asyncio
import json
import logging
import os
from datetime import datetime
from typing import Dict, List, Any, Optional
from dataclasses import dataclass, asdict
from aiohttp import web
import websockets
from websockets.server import serve, WebSocketServerProtocol
from pythonjsonlogger import jsonlogger

# Configure logging
LOG_LEVEL = os.getenv('LOG_LEVEL', 'INFO').upper()
logger = logging.getLogger(__name__)
logHandler = logging.StreamHandler()
formatter = jsonlogger.JsonFormatter()
logHandler.setFormatter(formatter)
logger.addHandler(logHandler)
logger.setLevel(LOG_LEVEL)


@dataclass
class Entity:
    """Represents a Home Assistant entity"""
    entity_id: str
    state: Any
    attributes: Dict[str, Any]
    last_changed: str = None
    last_updated: str = None

    def __post_init__(self):
        if self.last_changed is None:
            self.last_changed = datetime.utcnow().isoformat()
        if self.last_updated is None:
            self.last_updated = datetime.utcnow().isoformat()


@dataclass
class ServiceCall:
    """Represents a service call made to Home Assistant"""
    domain: str
    service: str
    service_data: Dict[str, Any]
    timestamp: str

    def to_dict(self):
        return asdict(self)


class MockHomeAssistant:
    """Mock Home Assistant service"""

    def __init__(self):
        self.entities: Dict[str, Entity] = {}
        self.service_calls: List[ServiceCall] = []
        self.websocket_clients: List[WebSocketServerProtocol] = []
        self.message_id = 0
        self.subscriptions: Dict[int, Dict] = {}

    def load_fixtures(self, fixtures_file: str):
        """Load initial entity states from fixtures file"""
        try:
            with open(fixtures_file, 'r') as f:
                data = json.load(f)

            for entity_data in data.get('entities', []):
                entity = Entity(
                    entity_id=entity_data['entity_id'],
                    state=entity_data['state'],
                    attributes=entity_data.get('attributes', {})
                )
                self.entities[entity.entity_id] = entity

            logger.info(f"Loaded {len(self.entities)} entities from fixtures")

        except Exception as e:
            logger.error(f"Failed to load fixtures: {e}")

    async def handle_websocket(self, websocket: WebSocketServerProtocol):
        """Handle WebSocket connection from homeautomation system"""
        logger.info(f"New WebSocket connection from {websocket.remote_address}")
        self.websocket_clients.append(websocket)

        try:
            # Send auth_required message
            await websocket.send(json.dumps({
                "type": "auth_required",
                "ha_version": "2024.1.0"
            }))

            # Wait for auth message
            auth_msg = await websocket.recv()
            auth_data = json.loads(auth_msg)

            if auth_data.get('type') == 'auth' and auth_data.get('access_token') == 'test_token_12345':
                await websocket.send(json.dumps({
                    "type": "auth_ok",
                    "ha_version": "2024.1.0"
                }))
                logger.info("Client authenticated successfully")
            else:
                await websocket.send(json.dumps({
                    "type": "auth_invalid",
                    "message": "Invalid access token"
                }))
                return

            # Handle messages
            async for message in websocket:
                await self.handle_message(websocket, message)

        except websockets.exceptions.ConnectionClosed:
            logger.info("WebSocket connection closed")
        except Exception as e:
            logger.error(f"WebSocket error: {e}")
        finally:
            if websocket in self.websocket_clients:
                self.websocket_clients.remove(websocket)

    async def handle_message(self, websocket: WebSocketServerProtocol, message: str):
        """Handle incoming WebSocket message"""
        try:
            data = json.loads(message)
            msg_type = data.get('type')
            msg_id = data.get('id')

            logger.debug(f"Received message type: {msg_type}")

            if msg_type == 'subscribe_events':
                await self.handle_subscribe_events(websocket, data, msg_id)
            elif msg_type == 'call_service':
                await self.handle_call_service(websocket, data, msg_id)
            elif msg_type == 'get_states':
                await self.handle_get_states(websocket, msg_id)
            elif msg_type == 'ping':
                await websocket.send(json.dumps({
                    "type": "pong",
                    "id": msg_id
                }))
            else:
                logger.warning(f"Unknown message type: {msg_type}")

        except json.JSONDecodeError as e:
            logger.error(f"Invalid JSON: {e}")
        except Exception as e:
            logger.error(f"Error handling message: {e}")

    async def handle_subscribe_events(self, websocket: WebSocketServerProtocol, data: Dict, msg_id: int):
        """Handle event subscription"""
        event_type = data.get('event_type')

        self.subscriptions[msg_id] = {
            'websocket': websocket,
            'event_type': event_type
        }

        await websocket.send(json.dumps({
            "type": "result",
            "success": True,
            "result": None,
            "id": msg_id
        }))

        logger.info(f"Client subscribed to event type: {event_type}")

    async def handle_call_service(self, websocket: WebSocketServerProtocol, data: Dict, msg_id: int):
        """Handle service call"""
        domain = data.get('domain')
        service = data.get('service')
        service_data = data.get('service_data', {})

        # Record the service call
        call = ServiceCall(
            domain=domain,
            service=service,
            service_data=service_data,
            timestamp=datetime.utcnow().isoformat()
        )
        self.service_calls.append(call)

        logger.info(f"Service call: {domain}.{service} with data: {service_data}")

        # Update entity state if applicable
        entity_id = service_data.get('entity_id')
        if entity_id:
            await self.update_entity_from_service(domain, service, entity_id, service_data)

        # Send success response
        await websocket.send(json.dumps({
            "type": "result",
            "success": True,
            "result": {
                "context": {
                    "id": "test_context_id",
                    "parent_id": None,
                    "user_id": None
                }
            },
            "id": msg_id
        }))

    async def update_entity_from_service(self, domain: str, service: str, entity_id: str, service_data: Dict):
        """Update entity state based on service call"""
        if entity_id not in self.entities:
            logger.warning(f"Entity {entity_id} not found")
            return

        entity = self.entities[entity_id]
        old_state = entity.state

        # Handle input_boolean
        if domain == 'input_boolean':
            if service == 'turn_on':
                entity.state = 'on'
            elif service == 'turn_off':
                entity.state = 'off'
            elif service == 'toggle':
                entity.state = 'off' if entity.state == 'on' else 'on'

        # Handle input_number
        elif domain == 'input_number' and service == 'set_value':
            entity.state = service_data.get('value')

        # Handle input_text
        elif domain == 'input_text' and service == 'set_value':
            entity.state = service_data.get('value')

        # Update timestamps
        entity.last_updated = datetime.utcnow().isoformat()
        if old_state != entity.state:
            entity.last_changed = entity.last_updated

            # Emit state_changed event
            await self.emit_state_changed(entity_id, old_state, entity.state)

    async def emit_state_changed(self, entity_id: str, old_state: Any, new_state: Any):
        """Emit state_changed event to all subscribers"""
        entity = self.entities.get(entity_id)
        if not entity:
            return

        event = {
            "type": "event",
            "event": {
                "event_type": "state_changed",
                "data": {
                    "entity_id": entity_id,
                    "old_state": {
                        "entity_id": entity_id,
                        "state": old_state,
                        "attributes": entity.attributes,
                        "last_changed": entity.last_changed,
                        "last_updated": entity.last_updated
                    },
                    "new_state": {
                        "entity_id": entity_id,
                        "state": new_state,
                        "attributes": entity.attributes,
                        "last_changed": entity.last_changed,
                        "last_updated": entity.last_updated
                    }
                },
                "origin": "LOCAL",
                "time_fired": datetime.utcnow().isoformat()
            }
        }

        # Send to all subscribed clients
        for sub_id, sub_info in self.subscriptions.items():
            if sub_info['event_type'] in [None, 'state_changed']:
                ws = sub_info['websocket']
                try:
                    await ws.send(json.dumps(event))
                    logger.debug(f"Emitted state_changed event for {entity_id}")
                except Exception as e:
                    logger.error(f"Failed to send event: {e}")

    async def handle_get_states(self, websocket: WebSocketServerProtocol, msg_id: int):
        """Handle get_states request"""
        states = []
        for entity in self.entities.values():
            states.append({
                "entity_id": entity.entity_id,
                "state": entity.state,
                "attributes": entity.attributes,
                "last_changed": entity.last_changed,
                "last_updated": entity.last_updated
            })

        await websocket.send(json.dumps({
            "type": "result",
            "success": True,
            "result": states,
            "id": msg_id
        }))

    async def inject_event(self, entity_id: str, new_state: Any):
        """Inject an external event (for testing)"""
        if entity_id not in self.entities:
            logger.warning(f"Cannot inject event for unknown entity: {entity_id}")
            return

        entity = self.entities[entity_id]
        old_state = entity.state
        entity.state = new_state
        entity.last_updated = datetime.utcnow().isoformat()
        entity.last_changed = entity.last_updated

        await self.emit_state_changed(entity_id, old_state, new_state)
        logger.info(f"Injected event: {entity_id} = {new_state}")

    def get_service_calls(self, domain: Optional[str] = None, service: Optional[str] = None) -> List[Dict]:
        """Get recorded service calls with optional filtering"""
        calls = self.service_calls

        if domain:
            calls = [c for c in calls if c.domain == domain]
        if service:
            calls = [c for c in calls if c.service == service]

        return [c.to_dict() for c in calls]

    def reset(self):
        """Reset recorded service calls and events"""
        self.service_calls = []
        logger.info("Reset service calls")


# Global instance
mock_ha = MockHomeAssistant()


# HTTP API for test control
async def health_check(request):
    """Health check endpoint"""
    return web.json_response({
        "status": "ready",
        "connected_clients": len(mock_ha.websocket_clients),
        "entities": len(mock_ha.entities)
    })


async def inject_event_handler(request):
    """Inject an event for testing"""
    try:
        data = await request.json()
        entity_id = data.get('entity_id')
        new_state = data.get('new_state')

        await mock_ha.inject_event(entity_id, new_state)

        return web.json_response({"success": True})

    except Exception as e:
        logger.error(f"Error injecting event: {e}")
        return web.json_response({"success": False, "error": str(e)}, status=500)


async def get_service_calls_handler(request):
    """Get recorded service calls"""
    domain = request.query.get('domain')
    service = request.query.get('service')

    calls = mock_ha.get_service_calls(domain, service)

    return web.json_response({"calls": calls})


async def get_entity_state_handler(request):
    """Get entity state"""
    entity_id = request.match_info['entity_id']
    entity = mock_ha.entities.get(entity_id)

    if not entity:
        return web.json_response({"error": "Entity not found"}, status=404)

    return web.json_response({
        "entity_id": entity.entity_id,
        "state": entity.state,
        "attributes": entity.attributes,
        "last_changed": entity.last_changed,
        "last_updated": entity.last_updated
    })


async def reset_handler(request):
    """Reset service calls and events"""
    mock_ha.reset()
    return web.json_response({"success": True})


async def main():
    """Main entry point"""
    # Load fixtures
    fixtures_file = os.getenv('FIXTURES_FILE', '/testdata/test_fixtures.json')
    mock_ha.load_fixtures(fixtures_file)

    # Start HTTP server for test control
    app = web.Application()
    app.router.add_get('/test/health', health_check)
    app.router.add_post('/test/inject_event', inject_event_handler)
    app.router.add_get('/test/service_calls', get_service_calls_handler)
    app.router.add_get('/test/entity/{entity_id}', get_entity_state_handler)
    app.router.add_post('/test/reset', reset_handler)

    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, '0.0.0.0', 8123)
    await site.start()

    logger.info("HTTP API started on port 8123")

    # Start WebSocket server
    async with serve(mock_ha.handle_websocket, '0.0.0.0', 8123, subprotocols=['websocket']):
        logger.info("WebSocket server started on port 8123")
        await asyncio.Future()  # Run forever


if __name__ == '__main__':
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Shutting down...")
