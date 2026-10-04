fn main() {
    tauri_build::try_build(tauri_build::Attributes::new().app_manifest(
        tauri_build::AppManifest::new().commands(&[
            "check_server",
            "start_hylo_server_detached",
            "stop_hylo_server",
            "get_server_process_status",
            "get_shell_debug_paths",
            "get_server_url",
            "set_server_url",
            "restart_hylo_server",
            "pick_folder",
            "set_view_bg_color",
            "set_window_button_visibility",
            "inbox_notify_get_settings",
            "inbox_notify_set_settings",
            "inbox_notify_preview_sound",
            "toggle_maximize",
        ]),
    ))
    .expect("failed to run tauri-build");
}
