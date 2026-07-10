#!/usr/bin/env node
import process from "node:process";
import {
	applyProfileDefaults,
	parseProfilePayload,
	profileSummary,
} from "./lib/profile-defaults.mjs";
import { loadDatabase } from "./lib/raycast-database.mjs";
import {
	addonSurface,
	databaseMethodSurface,
	databaseSummary,
	parseStoredJson,
	profileDefaults,
	resolveDatabaseMethod,
	userDefaultValue,
} from "./lib/raycast-introspection.mjs";

/**
 * @typedef {import("./lib/raycast-database.mjs").RaycastDatabaseContext} RaycastDatabaseContext
 */

function printUsage() {
	console.log(`Usage: node raycast-db.mjs <command> [options]

Commands:
  surface
      Print native addon exports and repository methods without opening the DB.
  status
      Print database initialization and health status.
  summary
      Print concise counts/settings.
  methods
      Print callable DatabaseClient and repository methods.
  call <method.path> [json-args] [--dry-run]
      Invoke any DatabaseClient/repository method with JSON positional args.
      Examples:
        node raycast-db.mjs call userDefaults.get '["CurrentUser"]'
        node raycast-db.mjs call userDefaults.set '["Key","Value"]'
        node raycast-db.mjs call settings.getGeneralSettings
  profile
      Print CurrentUser and OAuthTokenResponse user defaults.
  profile apply <current-user-json> <oauth-token-json> [--dry-run]
      Write CurrentUser and OAuthTokenResponse through the shared DB layer.
  user-default get <key>
      Read one Raycast user default, parsed as JSON when possible.
  user-default set <key> <value> [--dry-run]
      Set one Raycast user default as a raw string.
  user-default set-json <key> <json> [--dry-run]
      Set one Raycast user default as compact JSON.
  user-default delete <key> [--dry-run]
      Delete one Raycast user default.

Environment:
  RAYCAST_APP_SUPPORT
  RAYCAST_APP_BUNDLE
  RAYCAST_DATA_ADDON
  RAYCAST_KEY_FILE`);
}

/**
 * @param {string[]} argv
 * @param {string} flag
 * @returns {boolean}
 */
function hasFlag(argv, flag) {
	return argv.includes(flag);
}

/**
 * @param {string[]} argv
 * @returns {string[]}
 */
function withoutFlags(argv) {
	return argv.filter((arg) => !arg.startsWith("--"));
}

/**
 * @param {unknown} value
 * @returns {void}
 */
function printJson(value) {
	console.log(JSON.stringify(value, null, 2));
}

/**
 * @param {string | undefined} value
 * @returns {unknown[]}
 */
function parseJsonArgs(value) {
	if (value === undefined) return [];
	const parsed = JSON.parse(value);
	if (!Array.isArray(parsed)) {
		throw new Error("method args must be a JSON array");
	}
	return parsed;
}

/**
 * @template T
 * @param {(context: RaycastDatabaseContext) => Promise<T>} action
 * @returns {Promise<T>}
 */
async function withDatabase(action) {
	const context = await loadDatabase();
	try {
		return await action(context);
	} finally {
		await context.db.shutdown?.();
	}
}

/**
 * @param {string[]} argv
 * @returns {Promise<void>}
 */
async function runCall(argv) {
	const [methodPath, jsonArgs] = withoutFlags(argv);
	if (!methodPath) {
		throw new Error("usage: call <method.path> [json-args]");
	}

	const dryRun = hasFlag(argv, "--dry-run");
	const args = parseJsonArgs(jsonArgs);
	await withDatabase(async ({ db }) => {
		if (dryRun) {
			printJson({ dryRun, method: methodPath, args });
			return;
		}

		const { method, receiver } = resolveDatabaseMethod(db, methodPath);
		printJson({
			method: methodPath,
			args,
			result: await method.apply(receiver, args),
		});
	});
}

/**
 * @returns {Promise<void>}
 */
async function printStatus() {
	await withDatabase(async ({ db, appSupport, keyFile }) => {
		printJson({
			appSupport,
			keyFile,
			initReport: db.initReport,
			status: await db.getDatabaseStatus(),
		});
	});
}

/**
 * @param {string} value
 * @returns {string}
 */
function parseJsonArgument(value) {
	try {
		return JSON.stringify(JSON.parse(value));
	} catch (error) {
		throw new Error(`invalid JSON: ${error.message}`);
	}
}

/**
 * @param {string[]} argv
 * @returns {Promise<void>}
 */
async function runUserDefault(argv) {
	const [action, key, ...rest] = withoutFlags(argv);
	const dryRun = hasFlag(argv, "--dry-run");

	if (!action || !key) {
		throw new Error("usage: user-default <get|set|set-json|delete> <key>");
	}

	await withDatabase(async ({ db }) => {
		const before = await userDefaultValue(db, key);

		if (action === "get") {
			printJson({ key, value: before });
			return;
		}

		if (action === "delete") {
			if (!dryRun) await db.userDefaults.delete(key);
			printJson({
				dryRun,
				key,
				action,
				before,
				...(dryRun ? { plannedAfter: null } : { after: null }),
			});
			return;
		}

		const [value] = rest;
		if (value === undefined) {
			throw new Error(`usage: user-default ${action} <key> <value>`);
		}

		const storedValue =
			action === "set-json" ? parseJsonArgument(value) : value;
		if (action !== "set" && action !== "set-json") {
			throw new Error(`unknown user-default action: ${action}`);
		}

		if (!dryRun) await db.userDefaults.set(key, storedValue);
		const plannedAfter = parseStoredJson(storedValue);
		printJson({
			dryRun,
			key,
			action,
			before,
			...(dryRun
				? { plannedAfter }
				: { after: await userDefaultValue(db, key) }),
		});
	});
}

/**
 * @param {string[]} argv
 * @returns {Promise<void>}
 */
async function runProfile(argv) {
	const [action, ...rest] = withoutFlags(argv);
	const dryRun = hasFlag(argv, "--dry-run");

	if (!action) {
		await withDatabase(async ({ db }) => {
			printJson(await profileDefaults(db));
		});
		return;
	}

	if (action === "apply") {
		const [currentUser, oauthToken] = rest;
		const profile = parseProfilePayload(currentUser, oauthToken);
		if (dryRun) {
			printJson({ dryRun, profile });
			return;
		}

		await withDatabase(async ({ db }) => {
			const stored = await applyProfileDefaults(db, profile);
			printJson({ summary: profileSummary(stored), stored });
		});
		return;
	}

	throw new Error(`unknown profile action: ${action}`);
}

/**
 * @returns {Promise<void>}
 */
async function run() {
	const [command, ...argv] = process.argv.slice(2);
	if (!command || command === "--help" || command === "-h") {
		printUsage();
		return;
	}

	if (command === "surface") {
		printJson(addonSurface());
		return;
	}

	if (command === "status") {
		await printStatus();
		return;
	}

	if (command === "summary") {
		await withDatabase(async ({ db, appSupport, keyFile }) => {
			printJson({
				appSupport,
				keyFile,
				summary: await databaseSummary(db),
			});
		});
		return;
	}

	if (command === "methods") {
		await withDatabase(async ({ db }) => {
			printJson(databaseMethodSurface(db));
		});
		return;
	}

	if (command === "call") {
		await runCall(argv);
		return;
	}

	if (command === "profile") {
		await runProfile(argv);
		return;
	}

	if (command === "user-default") {
		await runUserDefault(argv);
		return;
	}

	throw new Error(`unknown command: ${command}`);
}

run().catch((error) => {
	const message = process.env.DEBUG
		? error.stack || error.message
		: error.message;
	console.error(`raycast-db: ${message}`);
	process.exitCode = 1;
});
