#!/usr/bin/env python
# coding:utf-8

"""
# -----------------------------------------------------------------------------
# ESSENTIAL PROCESS:
# Standalone Python client wrapper for notif-server via libnotif CGO bridge.
#
# DATA FLOW:
# Python App -> NotifClient.send -> libnotif (C ABI) -> gRPC Port 1027 -> notif-server
#
# KEY PARAMETERS:
# - target_addr: Target gRPC host:port of notif-server (default: 127.0.0.1:1027).
# - lib_path: Optional explicit filesystem path to libnotif shared library.
# -----------------------------------------------------------------------------
"""

import concurrent.futures
import ctypes
from ctypes import CDLL, c_char_p, c_double, c_int, c_void_p, string_at
import os
from pathlib import Path
import platform
import sys
from typing import Optional, Sequence

# Global executor for thread-safe CGO execution across OS platforms
_executor = concurrent.futures.ThreadPoolExecutor(max_workers=4)

class NotifError(Exception):
    """Exception raised when notification dispatch fails."""
    def __init__(self, message: str, code: int = 0):
        super().__init__(message)
        self.code = code

class NotifClient:
    """
    Python wrapper for libnotif shared library.
    Dispatches direct, multi-channel notifications to notif-server.
    """

    def __init__(self, target_addr: str = "127.0.0.1:1027", lib_path: Optional[str] = None):
        if not target_addr:
            raise ValueError("target_addr cannot be empty")
        self.target_addr = target_addr
        self.timeout = 5.0
        self._lib = self._load_lib(lib_path)
        if not self._lib:
            raise RuntimeError("Could not load libnotif shared library")
        self._closed = False

    def _load_lib(self, lib_path: Optional[str] = None) -> CDLL:
        if not lib_path:
            lib_path = os.getenv("LIBNOTIF_PATH")

        if not lib_path:
            sys_name = platform.system()
            ext = ".dylib" if sys_name == "Darwin" else (".dll" if sys_name == "Windows" else ".so")
            pkg_dir = Path(__file__).resolve().parent
            candidates = [
                pkg_dir.parent.parent / "libnotif" / f"libnotif{ext}",
                pkg_dir.parent / "libnotif" / f"libnotif{ext}",
                pkg_dir / f"libnotif{ext}",
                Path(f"libnotif{ext}"),
                Path(f"/usr/local/lib/libnotif{ext}"),
            ]
            for p in candidates:
                if p.exists():
                    lib_path = str(p.resolve())
                    break

        if not lib_path or not os.path.exists(lib_path):
            raise RuntimeError(f"libnotif shared library not found (searched: {lib_path})")

        handle = CDLL(lib_path)
        handle.Notif_Send.argtypes = [c_char_p, c_char_p, c_char_p, c_char_p, c_char_p]
        handle.Notif_Send.restype = c_int

        handle.Notif_SendWithTimeout.argtypes = [c_char_p, c_char_p, c_char_p, c_char_p, c_char_p, c_double]
        handle.Notif_SendWithTimeout.restype = c_int

        handle.Notif_GetLastError.argtypes = []
        handle.Notif_GetLastError.restype = c_void_p

        handle.Notif_FreeString.argtypes = [c_void_p]
        handle.Notif_FreeString.restype = None

        handle.Notif_Close.argtypes = []
        handle.Notif_Close.restype = None

        return handle

    def send(self, message: str, *tags: str) -> None:
        self.send_with_attachment("", message, "", *tags)

    def send_alert(self, level: str, message: str, *tags: str) -> None:
        self.send_with_attachment(level, message, "", *tags)

    def send_with_attachment(self, level: str, message: str, attachment: str = "", *tags: str) -> None:
        if self._closed:
            raise RuntimeError("client is closed")
        future = _executor.submit(self._dispatch, level, message, attachment, *tags)
        future.result()

    def _dispatch(self, level: str, message: str, attachment: str = "", *tags: str) -> None:
        tags_csv = ",".join(tags)
        target_b = self.target_addr.encode("utf-8")
        level_b = level.encode("utf-8")
        msg_b = message.encode("utf-8")
        att_b = attachment.encode("utf-8")
        tags_b = tags_csv.encode("utf-8")

        rc = self._lib.Notif_SendWithTimeout(
            target_b, level_b, msg_b, att_b, tags_b, c_double(self.timeout)
        )
        if rc != 0:
            err_ptr = self._lib.Notif_GetLastError()
            err_msg = ""
            if err_ptr:
                err_msg = string_at(err_ptr).decode("utf-8", errors="replace")
                self._lib.Notif_FreeString(err_ptr)
            raise NotifError(f"SendNotification failed: {err_msg}", code=rc)

    def close(self) -> None:
        self._closed = True
