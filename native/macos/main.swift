import AppKit
import UserNotifications

let args = Array(CommandLine.arguments.dropFirst())
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let center = UNUserNotificationCenter.current()
func finish(_ ok: Bool, _ message: String = "") {
    let result: [String: Any] = ["ok": ok, "error": message]
    if let destination = args.last, let data = try? JSONSerialization.data(withJSONObject: result) {
        try? data.write(to: URL(fileURLWithPath: destination), options: .atomic)
    }
    DispatchQueue.main.async { app.terminate(nil) }
}
final class Delegate: NSObject, UNUserNotificationCenterDelegate {
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler handler: @escaping (UNNotificationPresentationOptions) -> Void) {
        handler([.banner, .list])
    }
}
let delegate = Delegate()
center.delegate = delegate
if args.first == "authorize", args.count == 2 {
    center.requestAuthorization(options: [.alert]) { allowed, error in
        finish(allowed, error?.localizedDescription ?? (allowed ? "" : "Allow dfman notifications in System Settings."))
    }
} else if args.first == "send", args.count == 5 {
    center.getNotificationSettings { settings in
        guard settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional else {
            finish(false, "Allow dfman notifications in System Settings."); return
        }
        let content = UNMutableNotificationContent()
        content.title = args[1]
        content.body = args[2]
        let request = UNNotificationRequest(identifier: args[3], content: content, trigger: nil)
        center.add(request) { error in
            if let error { finish(false, error.localizedDescription); return }
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
                center.getDeliveredNotifications { notifications in
                    let delivered = notifications.contains { $0.request.identifier == args[3] }
                    finish(delivered, delivered ? "" : "Notification was accepted but delivery could not be verified.")
                }
            }
        }
    }
} else {
    finish(false, "Invalid notification request.")
}
DispatchQueue.main.asyncAfter(deadline: .now() + 90) { finish(false, "Notification request timed out.") }
app.run()
