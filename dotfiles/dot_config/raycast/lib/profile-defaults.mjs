/**
 * @typedef {import("./raycast-database.mjs").RaycastDatabaseClient} RaycastDatabaseClient
 *
 * @typedef {object} RaycastSubscription
 * @property {string} [id]
 * @property {string} [status]
 * @property {string} [plan_name]
 * @property {string} [billing_cycle]
 * @property {string} [renewal_date]
 *
 * @typedef {object} RaycastCurrentUser
 * @property {string} id
 * @property {string} name
 * @property {string} [username]
 * @property {string} [handle]
 * @property {string} [email]
 * @property {boolean} [has_pro_features]
 * @property {boolean} [can_apply_for_free_trial]
 * @property {RaycastSubscription} [subscription]
 * @property {string} [image]
 * @property {string} [avatar]
 *
 * @typedef {object} RaycastOAuthToken
 * @property {string} access_token
 * @property {string} [refresh_token]
 * @property {string} [token_type]
 * @property {number} [expires_in]
 * @property {string} [scope]
 *
 * @typedef {object} RaycastProfilePayload
 * @property {RaycastCurrentUser} currentUser
 * @property {RaycastOAuthToken} oauthToken
 */

export const PROFILE_USER_DEFAULTS = {
  currentUser: "CurrentUser",
  oauthToken: "OAuthTokenResponse",
};

/**
 * @param {string} currentUser
 * @param {string} oauthToken
 * @returns {RaycastProfilePayload}
 */
export function parseProfilePayload(currentUser, oauthToken) {
  if (!currentUser || !oauthToken) {
    throw new Error("current user and OAuth token JSON are required");
  }

  return {
    currentUser: JSON.parse(currentUser),
    oauthToken: JSON.parse(oauthToken),
  };
}

/**
 * @param {RaycastProfilePayload} profile
 * @returns {void}
 */
export function validateProfilePayload({ currentUser, oauthToken }) {
  if (!currentUser?.id || !currentUser?.name) {
    throw new Error("current user payload is missing id or name");
  }
  if (!oauthToken?.access_token) {
    throw new Error("OAuth token payload is missing access_token");
  }
}

/**
 * @param {RaycastDatabaseClient} db
 * @param {RaycastProfilePayload} profile
 * @returns {Promise<RaycastCurrentUser>}
 */
export async function applyProfileDefaults(db, profile) {
  validateProfilePayload(profile);

  await db.userDefaults.set(
    PROFILE_USER_DEFAULTS.currentUser,
    JSON.stringify(profile.currentUser),
  );
  await db.userDefaults.set(
    PROFILE_USER_DEFAULTS.oauthToken,
    JSON.stringify(profile.oauthToken),
  );

  const stored = await db.userDefaults.get(PROFILE_USER_DEFAULTS.currentUser);
  if (!stored) throw new Error("CurrentUser was not stored");
  return JSON.parse(stored);
}

/**
 * @param {RaycastCurrentUser} stored
 * @returns {string}
 */
export function profileSummary(stored) {
  return [
    `OK - ${stored.name}`,
    `pro:${stored.has_pro_features}`,
    `sub:${stored.subscription?.status}`,
  ].join(" | ");
}
