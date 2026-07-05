-- Copy selected or hovered files to the system clipboard.
--
-- Linux file managers disagree on clipboard metadata. The standard baseline is
-- text/uri-list; GNOME/Nautilus expects x-special/gnome-copied-files. macOS
-- gets newline-separated paths through pbcopy, since pbcopy only writes text.

local M = {}

M.title = "System Clipboard"

local state = ya.sync(function()
	local files = {}
	for _, file in pairs(cx.active.selected) do
		files[#files + 1] = file
	end

	if #files == 0 and cx.active.current.hovered then
		files[1] = cx.active.current.hovered
	end

	return files
end)

function M.notify(level, content)
	ya.notify({
		title = M.title,
		content = content,
		level = level,
		timeout = 5,
	})
end

function M.has_arg(args, name)
	for _, arg in ipairs(args or {}) do
		if arg == name or arg == "--" .. name then
			return true
		end
	end
	return false
end

function M.env(name)
	return os.getenv(name) or ""
end

function M.desktop()
	return table
		.concat({
			M.env("XDG_CURRENT_DESKTOP"),
			M.env("XDG_SESSION_DESKTOP"),
			M.env("DESKTOP_SESSION"),
			M.env("KDE_FULL_SESSION"),
			M.env("GNOME_DESKTOP_SESSION_ID"),
		}, " ")
		:lower()
end

function M.desktop_matches(desktop, names)
	for _, name in ipairs(names) do
		if desktop:find(name, 1, true) then
			return true
		end
	end
	return false
end

function M.wayland_backend(args)
	if M.has_arg(args, "paths") or M.has_arg(args, "text") then
		return "paths"
	elseif
		M.has_arg(args, "uris")
		or M.has_arg(args, "uri-list")
		or M.has_arg(args, "kde")
		or M.has_arg(args, "standard")
	then
		return "uri-list"
	elseif M.has_arg(args, "gnome") or M.has_arg(args, "nautilus") or M.has_arg(args, "files") then
		return "gnome"
	end

	local desktop = M.desktop()
	if M.desktop_matches(desktop, { "gnome", "cinnamon", "mate", "pantheon", "budgie", "unity" }) then
		return "gnome"
	elseif M.desktop_matches(desktop, { "kde", "plasma", "lxqt" }) then
		return "uri-list"
	else
		return "uri-list"
	end
end

function M.file_path(file)
	return tostring(file.path or file.url or file)
end

function M.percent_encode(path)
	if ya.percent_encode then
		return ya.percent_encode(path)
	end

	return path:gsub("([^%w%-%._~%!%$%&%'%(%)%*%+%,%;%=%:%@/])", function(char)
		return string.format("%%%02X", string.byte(char))
	end)
end

function M.file_uri(file)
	return "file://" .. M.percent_encode(M.file_path(file))
end

function M.join(lines, separator)
	return table.concat(lines, separator or "\n") .. (separator or "\n")
end

function M.status_error(status, err)
	if err then
		return tostring(err)
	elseif status and status.code then
		return "exit status " .. tostring(status.code)
	else
		return "unknown error"
	end
end

function M.write_stdin(command, args, payload)
	local child, err = Command(command):arg(args or {}):stdin(Command.PIPED):spawn()

	if not child then
		return nil, err
	end

	local ok, write_err = child:write_all(payload)
	if not ok then
		child:start_kill()
		return nil, write_err
	end

	ok, write_err = child:flush()
	if not ok then
		child:start_kill()
		return nil, write_err
	end

	ya.drop(child:take_stdin())
	return child:wait()
end

function M.copy_wayland_gnome(files, operation)
	local payload = { operation or "copy" }
	for _, file in ipairs(files) do
		payload[#payload + 1] = M.file_uri(file)
	end

	return M.write_stdin("wl-copy", { "--type", "x-special/gnome-copied-files" }, M.join(payload))
end

function M.copy_wayland_uri_list(files)
	local payload = {}
	for _, file in ipairs(files) do
		payload[#payload + 1] = M.file_uri(file)
	end

	return M.write_stdin("wl-copy", { "--type", "text/uri-list" }, M.join(payload, "\r\n"))
end

function M.copy_wayland_paths(files)
	local payload = {}
	for _, file in ipairs(files) do
		payload[#payload + 1] = M.file_path(file)
	end

	return M.write_stdin("wl-copy", { "--type", "text/plain;charset=utf-8" }, M.join(payload))
end

function M.copy_macos_text(files, uris)
	local payload = {}
	for _, file in ipairs(files) do
		payload[#payload + 1] = uris and M.file_uri(file) or M.file_path(file)
	end

	return M.write_stdin("pbcopy", {}, M.join(payload))
end

function M.copy(files, args)
	local uris = M.has_arg(args, "uris") or M.has_arg(args, "uri-list")

	if ya.target_os() == "macos" then
		local status, err = M.copy_macos_text(files, uris)
		if status and status.success then
			return status, "pbcopy"
		end
		return status, "pbcopy: " .. M.status_error(status, err)
	end

	local backend = M.wayland_backend(args)
	if backend == "paths" then
		local status, err = M.copy_wayland_paths(files)
		if status and status.success then
			return status, "wl-copy paths"
		end
		return status, "wl-copy paths: " .. M.status_error(status, err)
	end

	local operation = M.has_arg(args, "cut") and "cut" or "copy"
	if backend == "gnome" then
		local status, err = M.copy_wayland_gnome(files, operation)
		if status and status.success then
			return status, "wl-copy gnome-files"
		end

		local uri_status, uri_err = M.copy_wayland_uri_list(files)
		if uri_status and uri_status.success then
			return uri_status, "wl-copy uri-list"
		end

		return status, "wl-copy gnome-files: " .. M.status_error(status, err or uri_err)
	end

	local status, err = M.copy_wayland_uri_list(files)
	if status and status.success then
		return status, "wl-copy uri-list"
	end

	local gnome_status, gnome_err = M.copy_wayland_gnome(files, operation)
	if gnome_status and gnome_status.success then
		return gnome_status, "wl-copy gnome-files"
	end

	return status, "wl-copy uri-list: " .. M.status_error(status, err or gnome_err)
end

function M.entry(_, job)
	ya.emit("escape", { visual = true })

	local files = state()
	if #files == 0 then
		return M.notify("warn", "No file selected")
	end

	local status, backend = M.copy(files, job and job.args or {})
	if not status or not status.success then
		return M.notify("error", "Could not copy files: " .. tostring(backend))
	end

	local noun = #files == 1 and "file" or "files"
	M.notify("info", string.format("Copied %d %s with %s", #files, noun, backend))
end

return M
