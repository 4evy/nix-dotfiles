import { access, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

/**
 * @param {string | undefined} value
 * @returns {string | undefined}
 */
export function expandHome(value) {
	if (!value?.startsWith("~/")) return value;
	return path.join(os.homedir(), value.slice(2));
}

/**
 * @param {string} file
 * @returns {Promise<boolean>}
 */
export async function pathExists(file) {
	try {
		await access(file);
		return true;
	} catch {
		return false;
	}
}

/**
 * @param {string} file
 * @returns {Promise<unknown>}
 */
export async function readJson(file) {
	return JSON.parse(await readFile(file, "utf8"));
}

/**
 * @param {string} file
 * @param {unknown} value
 * @returns {Promise<void>}
 */
export async function writeJson(file, value) {
	await writeFile(file, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
}

/**
 * @template T
 * @param {T} value
 * @returns {T}
 */
export function clone(value) {
	return value == null ? value : structuredClone(value);
}

/**
 * @param {unknown} value
 * @returns {number}
 */
export function countRecords(value) {
	if (Array.isArray(value)) return value.length;
	if (value == null) return 0;
	return Object.keys(value).length;
}
