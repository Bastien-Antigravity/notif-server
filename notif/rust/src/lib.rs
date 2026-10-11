// =============================================================================
// ESSENTIAL PROCESS:
// Standalone Rust wrapper crate for notif-server, providing safe abstractions
// and FFI bindings to the libnotif CGO shared library.
//
// DATA FLOW:
// Rust Application -> NotificationClient -> libnotif FFI -> gRPC Port 1027 -> notif-server
//
// KEY PARAMETERS:
// - target_addr: Target gRPC address (host:port) of notif-server.
// - lib_path: Path to the compiled libnotif dynamic library.
// =============================================================================

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_double, c_int};
use libloading::{Library, Symbol};

pub struct NotificationClient {
    lib: &'static Library,
    target_addr: String,
    timeout: f64,
}

impl NotificationClient {
    /// Connects to a target notif-server instance using the libnotif dynamic library.
    pub fn new(target_addr: &str, lib_path: Option<&str>) -> Result<Self, Box<dyn std::error::Error>> {
        let path = if let Some(p) = lib_path {
            p.to_string()
        } else if let Ok(p) = std::env::var("LIBNOTIF_PATH") {
            p
        } else {
            let candidates = [
                "../libnotif/libnotif.dylib",
                "../libnotif/libnotif.so",
                "../../notif/libnotif/libnotif.dylib",
                "../../notif/libnotif/libnotif.so",
                "/usr/local/lib/libnotif.dylib",
                "/usr/local/lib/libnotif.so",
                "./libnotif.dylib",
                "./libnotif.so",
            ];
            let mut found = "../libnotif/libnotif.dylib".to_string();
            for c in candidates {
                if std::path::Path::new(c).exists() {
                    found = c.to_string();
                    break;
                }
            }
            found
        };

        let lib = Box::leak(Box::new(unsafe { Library::new(path)? }));

        Ok(Self {
            lib,
            target_addr: target_addr.to_string(),
            timeout: 5.0,
        })
    }

    /// Sets default RPC timeout in seconds.
    pub fn set_timeout(&mut self, timeout: f64) {
        self.timeout = timeout;
    }

    /// Dispatches an untyped notification message with optional tags.
    pub fn send(&self, message: &str, tags: &[&str]) -> Result<(), String> {
        self.send_with_attachment("", message, "", tags)
    }

    /// Dispatches a categorized alert with level and optional tags.
    pub fn send_alert(&self, level: &str, message: &str, tags: &[&str]) -> Result<(), String> {
        self.send_with_attachment(level, message, "", tags)
    }

    /// Dispatches a notification with level, message, attachment, and tags.
    pub fn send_with_attachment(
        &self,
        level: &str,
        message: &str,
        attachment: &str,
        tags: &[&str],
    ) -> Result<(), String> {
        let target_c = CString::new(self.target_addr.as_str()).map_err(|e| e.to_string())?;
        let level_c = CString::new(level).map_err(|e| e.to_string())?;
        let message_c = CString::new(message).map_err(|e| e.to_string())?;
        let attachment_c = CString::new(attachment).map_err(|e| e.to_string())?;
        let tags_csv = tags.join(",");
        let tags_c = CString::new(tags_csv).map_err(|e| e.to_string())?;

        unsafe {
            let func: Symbol<extern "C" fn(
                *const c_char,
                *const c_char,
                *const c_char,
                *const c_char,
                *const c_char,
                c_double,
            ) -> c_int> = self.lib.get(b"Notif_SendWithTimeout").map_err(|e| e.to_string())?;

            let rc = func(
                target_c.as_ptr(),
                level_c.as_ptr(),
                message_c.as_ptr(),
                attachment_c.as_ptr(),
                tags_c.as_ptr(),
                self.timeout,
            );

            if rc != 0 {
                let err_fn: Symbol<unsafe extern "C" fn() -> *const c_char> =
                    self.lib.get(b"Notif_GetLastError").map_err(|e| e.to_string())?;
                let free_fn: Symbol<extern "C" fn(*mut c_char)> =
                    self.lib.get(b"Notif_FreeString").map_err(|e| e.to_string())?;

                let ptr = err_fn();
                let err_msg = if !ptr.is_null() {
                    let s = CStr::from_ptr(ptr).to_string_lossy().into_owned();
                    free_fn(ptr as *mut c_char);
                    s
                } else {
                    format!("error rc={}", rc)
                };

                return Err(format!("SendNotification failed: {}", err_msg));
            }
        }

        Ok(())
    }
}
