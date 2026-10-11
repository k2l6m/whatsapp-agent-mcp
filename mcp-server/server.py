from mcp.server.mcpserver import MCPServer
import requests

server = MCPServer(
    name="WhatsApp"
)

BASE_URL = "http://localhost:8080/api"


def bridge_available() -> bool:
    """Check if the Go backend REST API is reachable."""
    try:
        # Using contact search with an empty query as a lightweight health/readiness ping
        r = requests.get(
            f"{BASE_URL}/contacts/search",
            params={"name": ""},
            timeout=3,
        )
        return r.status_code == 200
    except Exception:
        return False


@server.tool(
    description="Check whether the WhatsApp MCP server and backend bridge are running and reachable."
)
def health_check() -> dict:
    is_bridge_up = bridge_available()
    return {
        "status": "ok",
        "bridge": "connected" if is_bridge_up else "disconnected"
    }


@server.tool(description="""
    Send a WhatsApp text message or media file to a recipient JID or phone number.
    Can also reply/quote an existing message if quoted_message_id is provided.
""")
def send_whatsapp_message(
    recipient: str, 
    message: str = "", 
    media_path: str = "", 
    quoted_message_id: str = "", 
    quoted_participant: str = ""
) -> dict:
    if not bridge_available():
        return {"success": False, "message": "WhatsApp service backend is not running."}

    payload = {
        "recipient": recipient,
        "message": message,
        "media_path": media_path,
        "quoted_message_id": quoted_message_id,
        "quoted_participant": quoted_participant,
    }

    try:
        response = requests.post(
            f"{BASE_URL}/send",
            json=payload,
            timeout=15,
        )
        return response.json()
    except Exception as e:
        return {"success": False, "error": str(e)}


@server.tool(description="""
    Send a file/media through WhatsApp.
    Provide recipient phone number/JID and the absolute local file path.
""")
def send_file(
    recipient: str,
    media_path: str,
) -> dict:
    if not bridge_available():
        return {
            "success": False,
            "message": "WhatsApp service backend is not running."
        }

    response = requests.post(
        f"{BASE_URL}/send",
        json={
            "recipient": recipient,
            "media_path": media_path,
        },
        timeout=60,
    )
    return response.json()


@server.tool(description="""
    Search WhatsApp contacts by name, push name, or phone number from the local database.
    Useful for finding correct JIDs or phone numbers before messaging.
""")
def search_contacts(query: str) -> dict:
    if not bridge_available():
        return {
            "success": False,
            "message": "WhatsApp service backend is not running."
        }

    try:
        response = requests.get(
            f"{BASE_URL}/contacts/search",
            params={"query": query},
            timeout=10,
        )
        return {"success": True, "contacts": response.json()}
    except Exception as e:
        return {"success": False, "error": str(e)}


@server.tool(description="""
    Check if a phone number is registered on WhatsApp and get its official JID.
    Useful for verifying unsaved contacts before sending messages.
    Provide international phone number format (e.g., 919876543210 or +919876543210).
""")
def check_whatsapp_number(phone: str) -> dict:
    if not bridge_available():
        return {
            "success": False,
            "message": "WhatsApp service backend is not running."
        }

    try:
        response = requests.get(
            f"{BASE_URL}/check-number",
            params={"phone": phone},
            timeout=10,
        )
        return response.json()
    except Exception as e:
        return {"success": False, "error": str(e)}


@server.tool(description="""
    Download media (images, videos, audio, documents) received in a chat 
    using the message ID and chat JID.
""")
def download_media(message_id: str, chat_jid: str) -> dict:
    if not bridge_available():
        return {
            "success": False,
            "message": "WhatsApp service backend is not running."
        }

    response = requests.post(
        f"{BASE_URL}/download",
        json={
            "message_id": message_id,
            "chat_jid": chat_jid,
        },
        timeout=60,
    )
    return response.json()


@server.tool(description="List recent active WhatsApp chats and their last message timestamps.")
def list_recent_chats() -> dict:
    if not bridge_available():
        return {"success": False, "message": "WhatsApp service backend is not running."}
    try:
        response = requests.get(f"{BASE_URL}/chats/recent", timeout=10)
        return {"success": True, "chats": response.json()}
    except Exception as e:
        return {"success": False, "error": str(e)}

@server.tool(description="Get recent message history for a specific WhatsApp chat JID.")
def get_chat_history(jid: str) -> dict:
    if not bridge_available():
        return {"success": False, "message": "WhatsApp service backend is not running."}
    try:
        response = requests.get(f"{BASE_URL}/chat/history", params={"jid": jid}, timeout=10)
        return {"success": True, "messages": response.json()}
    except Exception as e:
        return {"success": False, "error": str(e)}

if __name__ == "__main__":
    print("Starting WhatsApp MCP Server...")
    print("Waiting for MCP client connection...")
    server.run()