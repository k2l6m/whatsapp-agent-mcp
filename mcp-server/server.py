from mcp.server.mcpserver import MCPServer
import requests

server = MCPServer(
    name="WhatsApp"
)

BASE_URL = "http://localhost:8080/api"


@server.tool()
def send_whatsapp(
    recipient: str,
    message: str,
) -> dict:

    response = requests.post(
        f"{BASE_URL}/send",
        json={
            "recipient": recipient,
            "message": message,
        },
        timeout=30,
    )

    return response.json()


@server.tool()
def send_file(
    recipient: str,
    media_path: str,
) -> dict:

    response = requests.post(
        f"{BASE_URL}/send",
        json={
            "recipient": recipient,
            "media_path": media_path,
        },
        timeout=60,
    )

    return response.json()


if __name__ == "__main__":
	print("Starting WhatsApp MCP Server...")
	print("Waiting for MCP client connection...")
	server.run()