import { exec } from "node:child_process";
import { promisify } from "node:util";
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";

const execAsync = promisify(exec);

async function isDarkMode(): Promise<boolean> {
  if (process.platform === "darwin") {
    try {
      await execAsync("defaults read -g AppleInterfaceStyle");
      return true;
    } catch {
      return false;
    }
  }

  try {
    const { stdout } = await execAsync(
      "gsettings get org.gnome.desktop.interface color-scheme",
    );
    return stdout.toLowerCase().includes("dark");
  } catch {
    return true;
  }
}

export default function (pi: ExtensionAPI) {
  let intervalId: ReturnType<typeof setInterval> | null = null;

  pi.on("session_start", async (_event, ctx) => {
    let currentTheme = (await isDarkMode())
      ? "catppuccin-frappe-pink"
      : "catppuccin-latte-pink";
    ctx.ui.setTheme(currentTheme);

    intervalId = setInterval(async () => {
      const nextTheme = (await isDarkMode())
        ? "catppuccin-frappe-pink"
        : "catppuccin-latte-pink";
      if (nextTheme !== currentTheme) {
        currentTheme = nextTheme;
        ctx.ui.setTheme(currentTheme);
      }
    }, 2000);
  });

  pi.on("session_shutdown", () => {
    if (intervalId) {
      clearInterval(intervalId);
      intervalId = null;
    }
  });
}
