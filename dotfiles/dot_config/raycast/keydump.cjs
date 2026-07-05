const Module = require("module");
const originalRequire = Module.prototype.require;

Module.prototype.require = function requireWithKeyDump(id) {
  const result = originalRequire.apply(this, arguments);

  if (id && id.includes("data.darwin-arm64") && result.DatabaseClient) {
    const OriginalDatabaseClient = result.DatabaseClient;

    result.DatabaseClient = class DatabaseClientWithKeyDump extends (
      OriginalDatabaseClient
    ) {
      constructor(...args) {
        require("fs").writeFileSync(__KEY_FILE_JSON__, args[1]);
        super(...args);
      }
    };
  }

  return result;
};
