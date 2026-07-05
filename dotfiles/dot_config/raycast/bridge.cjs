async function main() {
  const [appSupport, key, nativeAddon, currentUser, oauthToken] =
    process.argv.slice(2);

  const raycastData = require(nativeAddon);
  const db = new raycastData.DatabaseClient(appSupport, key, () => {});

  if (!db.initReport || !db.initReport.overallSuccess) {
    throw new Error("failed to open Raycast database with extracted key");
  }

  await db.userDefaults.set("CurrentUser", currentUser);
  await db.userDefaults.set("OAuthTokenResponse", oauthToken);

  const stored = JSON.parse(await db.userDefaults.get("CurrentUser"));
  const summary = [
    "OK - " + stored.name,
    "pro:" + stored.has_pro_features,
    "sub:" + stored.subscription?.status,
  ].join(" | ");

  console.log(summary);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
