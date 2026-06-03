from __future__ import annotations

import subprocess
import threading

from agent_worker.contracts import utc_now_iso
from agent_worker.events import emit_worker_heartbeat, emit_worker_log, emit_worker_stream


def run_command_with_events(
    command: list[str],
    cwd: str,
    timeout: int,
    env: dict[str, str],
    label: str,
) -> subprocess.CompletedProcess[str]:
    emit_worker_log("info", f"spawn command label={label}", utc_now_iso())
    emit_worker_heartbeat("running", f"{label} started", utc_now_iso())
    process = subprocess.Popen(
        command,
        cwd=cwd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        errors="replace",
        bufsize=1,
        env=env,
    )
    stdout_chunks: list[str] = []
    stderr_chunks: list[str] = []
    stop_heartbeat = threading.Event()

    def pump(stream, sink: list[str], stream_name: str) -> None:
        try:
            for raw in iter(stream.readline, ""):
                if raw == "":
                    break
                sink.append(raw)
                text = raw.strip()
                if text:
                    emit_worker_stream(stream_name, text, utc_now_iso())
        finally:
            stream.close()

    def heartbeat_loop() -> None:
        while not stop_heartbeat.wait(5):
            emit_worker_heartbeat("running", f"{label} still running", utc_now_iso())

    stdout_thread = threading.Thread(target=pump, args=(process.stdout, stdout_chunks, "stdout"), daemon=True)
    stderr_thread = threading.Thread(target=pump, args=(process.stderr, stderr_chunks, "stderr"), daemon=True)
    pulse_thread = threading.Thread(target=heartbeat_loop, daemon=True)
    stdout_thread.start()
    stderr_thread.start()
    pulse_thread.start()
    try:
        returncode = process.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        process.kill()
        raise
    finally:
        stop_heartbeat.set()
        stdout_thread.join(timeout=1)
        stderr_thread.join(timeout=1)
        pulse_thread.join(timeout=1)
    return subprocess.CompletedProcess(
        args=command,
        returncode=returncode,
        stdout="".join(stdout_chunks),
        stderr="".join(stderr_chunks),
    )
