import json
import os
import sys
import tempfile
from datetime import datetime

HOME = os.path.expanduser("~")
SCION_JSON_PATH = os.path.join(HOME, "agent-info.json")
AGENT_LOG_PATH = os.path.join(HOME, "agent.log")

def log_event(state, message):
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    with open(AGENT_LOG_PATH, "a") as f:
        f.write(f"{timestamp} [{state}] {message}\n")

def update_status(status, session=False):
    data = {}
    if os.path.exists(SCION_JSON_PATH):
        try:
            with open(SCION_JSON_PATH, "r") as f:
                data = json.load(f)
        except Exception as e:
            log_event("ERROR", f"Failed to read {SCION_JSON_PATH}: {e}")
    
    key = "sessionStatus" if session else "status"
    data[key] = status

    try:
        # Atomic write
        fd, temp_path = tempfile.mkstemp(dir=os.path.dirname(SCION_JSON_PATH))
        with os.fdopen(fd, 'w') as f:
            json.dump(data, f, indent=2)
        os.replace(temp_path, SCION_JSON_PATH)
    except Exception as e:
        log_event("ERROR", f"Failed to update {os.path.basename(SCION_JSON_PATH)}: {e}")

def update_session_status(status):
    # Compatibility wrapper or just use update_status(status, session=True)
    update_status(status, session=True)

def ask_user(message):
    update_status("WAITING_FOR_INPUT", session=True)
    log_event("WAITING_FOR_INPUT", f"Agent requested input: {message}")
    print(f"Agent asked: {message}")

def task_completed(message):
    update_status("COMPLETED", session=True)
    log_event("COMPLETED", f"Agent completed task: {message}")
    print(f"Agent completed: {message}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 scion_tool.py <command> [args...]")
        sys.exit(1)

    command = sys.argv[1]

    if command == "ask_user":
        message = " ".join(sys.argv[2:]) if len(sys.argv) > 2 else "Input requested"
        ask_user(message)
    elif command == "task_completed":
        message = " ".join(sys.argv[2:]) if len(sys.argv) > 2 else "Task completed"
        task_completed(message)
    else:
        print(f"Unknown command: {command}")
        sys.exit(1)