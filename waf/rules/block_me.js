if (req.ip === "127.0.0.1" || req.ip === "::1") {
  return helpers.block();
}