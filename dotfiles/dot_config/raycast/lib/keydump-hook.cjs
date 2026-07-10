const fs = require("node:fs");
const Module = require("node:module");

const PATCHED = Symbol.for("dotfiles.raycast.keydumpHookPatched");

/**
 * Hooks Raycast's native data addon and writes the encryption key passed to
 * DatabaseClient into a text cache file.
 *
 * @param {string | undefined} [keyFile]
 * @returns {void}
 */
function installDatabaseKeyDump(keyFile = process.env.RAYCAST_KEYDUMP_FILE) {
	if (!keyFile) {
		throw new Error("RAYCAST_KEYDUMP_FILE is required");
	}
	if (Module.prototype[PATCHED]) return;

	const originalRequire = Module.prototype.require;

	Module.prototype.require = function requireWithKeyDump(id, ...args) {
		const result = originalRequire.call(this, id, ...args);

		if (id?.includes("data.darwin-arm64") && result.DatabaseClient) {
			const OriginalDatabaseClient = result.DatabaseClient;

			result.DatabaseClient = class DatabaseClientWithKeyDump extends (
				OriginalDatabaseClient
			) {
				constructor(...args) {
					fs.writeFileSync(keyFile, args[1]);
					super(...args);
				}
			};
		}

		return result;
	};

	Module.prototype[PATCHED] = true;
}

module.exports = {
	installDatabaseKeyDump,
};
