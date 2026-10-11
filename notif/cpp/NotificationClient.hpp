// -----------------------------------------------------------------------------
// ESSENTIAL PROCESS:
// C++ RAII facade and header-only wrapper for notif-server direct alert dispatch,
// binding to the libnotif CGO shared object.
//
// DATA FLOW:
// C++ Application -> NotificationClient -> libnotif C ABI -> gRPC (Port 1027) -> notif-server
//
// KEY PARAMETERS:
// - target_addr: Target gRPC address (host:port) of notif-server.
// -----------------------------------------------------------------------------

#ifndef NOTIF_NOTIFICATION_CLIENT_HPP
#define NOTIF_NOTIFICATION_CLIENT_HPP

#include <string>
#include <vector>
#include <stdexcept>
#include <sstream>

#include "../libnotif/libnotif.h"

namespace notif {

class NotificationClient {
public:
    explicit NotificationClient(const std::string &target_addr)
        : target_addr_(target_addr), timeout_sec_(5.0), closed_(false) {
        if (target_addr_.empty()) {
            throw std::invalid_argument("target_addr cannot be empty");
        }
    }

    ~NotificationClient() {
        closed_ = true;
    }

    void SetTimeout(double seconds) {
        if (seconds > 0) {
            timeout_sec_ = seconds;
        }
    }

    bool Send(const std::string &message, const std::vector<std::string> &tags = {}) {
        return SendWithAttachment("", message, "", tags);
    }

    bool SendAlert(const std::string &level, const std::string &message, const std::vector<std::string> &tags = {}) {
        return SendWithAttachment(level, message, "", tags);
    }

    bool SendWithAttachment(const std::string &level, const std::string &message,
                            const std::string &attachment,
                            const std::vector<std::string> &tags = {}) {
        if (closed_) {
            last_error_ = "NotificationClient is closed";
            return false;
        }

        std::string tagsCSV;
        for (size_t i = 0; i < tags.size(); ++i) {
            if (i > 0) tagsCSV += ",";
            tagsCSV += tags[i];
        }

        int rc = Notif_SendWithTimeout(
            const_cast<char*>(target_addr_.c_str()),
            const_cast<char*>(level.c_str()),
            const_cast<char*>(message.c_str()),
            const_cast<char*>(attachment.c_str()),
            const_cast<char*>(tagsCSV.c_str()),
            timeout_sec_
        );

        if (rc != 0) {
            char *err = Notif_GetLastError();
            if (err) {
                last_error_ = err;
                Notif_FreeString(err);
            } else {
                last_error_ = "Unknown RPC error (rc=" + std::to_string(rc) + ")";
            }
            return false;
        }

        last_error_.clear();
        return true;
    }

    void Close() {
        closed_ = true;
    }

    const std::string &TargetAddr() const { return target_addr_; }
    const std::string &LastError() const { return last_error_; }
    bool IsClosed() const { return closed_; }

private:
    std::string target_addr_;
    double timeout_sec_;
    bool closed_;
    std::string last_error_;
};

} // namespace notif

#endif // NOTIF_NOTIFICATION_CLIENT_HPP
