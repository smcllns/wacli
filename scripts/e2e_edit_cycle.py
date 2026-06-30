#!/usr/bin/env python3
import argparse
import json
import socket
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any


@dataclass(frozen=True)
class EditCyclePlan:
    token: str
    original_text: str
    first_edit_text: str
    second_edit_text: str


def build_plan(token: str) -> EditCyclePlan:
    return EditCyclePlan(
        token=token,
        original_text=f"WACLI_EDIT_E2E_{token}_original",
        first_edit_text=f"WACLI_EDIT_E2E_{token}_edit1",
        second_edit_text=f"WACLI_EDIT_E2E_{token}_edit2",
    )


def event_matches_text(event: dict[str, Any], expected_text: str, expected_chat: str | None) -> bool:
    if event.get("type") != "message":
        return False
    if expected_chat and event.get("chatJid") != expected_chat:
        return False
    return event.get("text") == expected_text or event.get("displayText") == expected_text or expected_text in str(event.get("displayText", ""))


def response_data(line: bytes) -> dict[str, Any]:
    payload = json.loads(line.decode("utf-8"))
    if payload.get("type") != "response":
        raise RuntimeError(f"expected response, got {payload}")
    if not payload.get("success"):
        raise RuntimeError(str(payload.get("error", payload)))
    data = payload.get("data")
    if not isinstance(data, dict):
        raise RuntimeError(f"response data is not an object: {payload}")
    return data


def send_command(socket_path: str, command: dict[str, Any], timeout: float) -> dict[str, Any]:
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.settimeout(timeout)
        sock.connect(socket_path)
        sock.sendall(json.dumps(command).encode("utf-8") + b"\n")
        return response_data(sock.makefile("rb").readline())


def require_capabilities(socket_path: str, capabilities: list[str], timeout: float) -> dict[str, Any]:
    health = send_command(socket_path, {"type": "health"}, timeout)
    present = health.get("capabilities")
    if not isinstance(present, list):
        raise RuntimeError(f"daemon health missing capabilities: {health}")
    missing = [cap for cap in capabilities if cap not in present]
    if missing:
        raise RuntimeError(f"daemon missing capabilities {missing}: {present}")
    return health


def subscribe(socket_path: str, timeout: float) -> tuple[socket.socket, Any]:
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.settimeout(timeout)
    sock.connect(socket_path)
    file = sock.makefile("rb")
    sock.sendall(b'{"type":"subscribe"}\n')
    response_data(file.readline())
    return sock, file


def wait_for_event(sock: socket.socket, file: Any, expected_text: str, expected_chat: str | None, deadline: float) -> dict[str, Any]:
    while time.time() < deadline:
        remaining = max(0.1, deadline - time.time())
        sock.settimeout(remaining)
        line = file.readline()
        if not line:
            raise RuntimeError("receiver subscription closed before expected event")
        payload = json.loads(line.decode("utf-8"))
        if event_matches_text(payload, expected_text, expected_chat):
            return payload
    raise RuntimeError(f"timed out waiting for receiver event containing {expected_text!r}")


def run(args: argparse.Namespace) -> dict[str, Any]:
    plan = build_plan(args.token)
    sender_health = require_capabilities(args.sender_socket, ["send_text", "send_edit"], args.timeout)
    receiver_health = require_capabilities(args.receiver_socket, [], args.timeout)

    if not args.go:
        return {
            "ready": True,
            "dryRun": True,
            "senderSocket": args.sender_socket,
            "receiverSocket": args.receiver_socket,
            "sendChat": args.send_chat,
            "expectChat": args.expect_chat,
            "originalText": plan.original_text,
            "firstEditText": plan.first_edit_text,
            "secondEditText": plan.second_edit_text,
            "senderHealth": sender_health,
            "receiverHealth": receiver_health,
            "note": "Pass --go to send one text plus two edits and verify receiver events.",
        }

    receiver_sock, receiver_file = subscribe(args.receiver_socket, args.timeout)
    try:
        deadline = time.time() + args.wait_seconds
        send_data = send_command(args.sender_socket, {"type": "send_text", "chatJid": args.send_chat, "message": plan.original_text}, args.timeout)
        original_id = str(send_data.get("message_id", ""))
        if not original_id:
            raise RuntimeError(f"send_text response missing message_id: {send_data}")
        original_event = wait_for_event(receiver_sock, receiver_file, plan.original_text, args.expect_chat, deadline)

        first_edit_data = send_command(args.sender_socket, {"type": "send_edit", "chatJid": args.send_chat, "msgId": original_id, "message": plan.first_edit_text}, args.timeout)
        first_edit_event = wait_for_event(receiver_sock, receiver_file, plan.first_edit_text, args.expect_chat, deadline)

        second_edit_data = send_command(args.sender_socket, {"type": "send_edit", "chatJid": args.send_chat, "msgId": original_id, "message": plan.second_edit_text}, args.timeout)
        second_edit_event = wait_for_event(receiver_sock, receiver_file, plan.second_edit_text, args.expect_chat, deadline)

        return {
            "success": True,
            "originalMsgId": original_id,
            "sendData": send_data,
            "firstEditData": first_edit_data,
            "secondEditData": second_edit_data,
            "originalEvent": original_event,
            "firstEditEvent": first_edit_event,
            "secondEditEvent": second_edit_event,
        }
    finally:
        receiver_sock.close()


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run a bounded two-device WhatsApp edit-cycle E2E over wacli daemon sockets.")
    parser.add_argument("--sender-socket", required=True)
    parser.add_argument("--receiver-socket", required=True)
    parser.add_argument("--send-chat", required=True, help="Recipient chat JID from the sender device's perspective")
    parser.add_argument("--expect-chat", help="Expected chat JID in receiver events, usually the sender's user JID for DM tests")
    parser.add_argument("--token", default=datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S"))
    parser.add_argument("--timeout", type=float, default=10.0)
    parser.add_argument("--wait-seconds", type=float, default=60.0)
    parser.add_argument("--go", action="store_true", help="Actually send one text plus two edits")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    try:
        result = run(parse_args(argv))
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
